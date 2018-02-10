package main

import (
	"bytes"
	"crypto/rand"
	"net/http"
	"log"
	"fmt"
	"io/ioutil"
	"time"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"io"
	"os"
	"flag"
	"strings"
)

func main() {
	logForwarder := NewS3LogForwarder()
	logForwarder.serve()
}

func NewS3LogForwarder() *S3LogForwarder {
	result := new(S3LogForwarder)
	result.channel = make(chan []byte, 2048)

	flag.BoolVar(&result.verbose, "verbose", false, "log level")

	flag.BoolVar(&result.https, "https", false, "listen using")
	flag.BoolVar(&result.https, "server-certificate", false, "listen using")
	flag.IntVar(&result.listenPort, "listen-port", 4443, "of the log forwarder")
	flag.StringVar(&result.listenAddress, "listen-address", "0.0.0.0", "of the log forwarder")

	flag.StringVar(&result.bucketName, "bucket-name", "", "to write to")
	flag.StringVar(&result.bucketRegion, "region", os.Getenv("AWS_REGION"), "of the s3 bucket")
	flag.StringVar(&result.keyPrefix, "key-prefix", "", "for the s3 bucket key")

	flag.DurationVar(&result.flushPeriod, "period", time.Duration(time.Second*30), "between flushesto s3")
	flag.IntVar(&result.cacheSize, "cache-size", 4096, "")

	flag.Parse()
	if result.bucketName == "" {
		log.Fatal("no bucket name specified.")
	}
	if result.bucketRegion == "" {
		log.Fatal("no bucket region specified.")
	}

	if result.cacheSize <= 0 {
		log.Fatal("cache size cannot be less than 0")
	}

	if result.flushPeriod <= time.Duration(time.Second*0) {
		log.Fatal("flush period cannot be less than 0")
	}

	result.keyPrefix = strings.TrimRight(result.keyPrefix, "/")

	sess, err := session.NewSession()
	if err != nil {
		log.Fatal("sessionNewSession", err)
	}
	result.s3 = s3.New(sess, &aws.Config{Region: aws.String(result.bucketRegion)})

	return result
}

func (f *S3LogForwarder) serve() {
	go f.bufferedPut()

	if f.verbose {
		log.Printf("forwarding to s3 bucket %s in %s\n", f.bucketName, f.bucketRegion)
		log.Printf("flush period: %s, cache size %d\n", f.flushPeriod.String(), f.cacheSize)
	}

	http.HandleFunc("/", f.KongLogForwarder)
	err := http.ListenAndServeTLS(fmt.Sprintf("%s:%d", f.listenAddress, f.listenPort), "server.crt", "server.key", nil)

	close(f.channel)

	if err != nil {
		log.Fatal("ListenAndServe", err)
	}
}

func (f *S3LogForwarder) KongLogForwarder(w http.ResponseWriter, req *http.Request) {
	if req.Method == "POST" {
		body, err := ioutil.ReadAll(req.Body)
		if err == nil {
			f.channel <- body
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprintf(w, "%d bytes\n", len(body))
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	} else {
		http.Error(w, "unsupported HTTP method", http.StatusBadRequest)
	}
}

func (f *S3LogForwarder) bufferedPut() {
	var buffer bytes.Buffer
	timer := time.NewTimer(f.flushPeriod)
	for {
		select {
		case body, more := <-f.channel:
			buffer.Write(body)
			if body[len(body)-1] != 10 {
				buffer.Write([]byte("\n"))
			}
			if !more || buffer.Len() >= f.cacheSize {
				f.putObject(&buffer)
			}
			if !more {
				return
			}
		case <-timer.C:
			f.putObject(&buffer)
			timer.Reset(f.flushPeriod)
		}
	}
}

func (f *S3LogForwarder) putObject(buffer *bytes.Buffer) {
	if buffer.Len() > 0 {
		now := time.Now()
		uuid := newUUID()
		hostName, _ := os.Hostname()
		key := fmt.Sprintf("%s/%04d/%02d/%02d/%04d%02d%02dT%02d%02d%02d.%06dZ-%s-%x.log",
			f.keyPrefix, now.Year(), now.Month(), now.Day(), now.Year(), now.Month(), now.Day(),
			now.Hour(), now.Minute(), now.Second(), now.Nanosecond(), hostName, uuid)
		if f.verbose {
			fmt.Printf("writing buffer of %d bytes to %s\n", buffer.Len(), key)
		}
		request := s3.PutObjectInput{
			Bucket:          &f.bucketName,
			Body:            bytes.NewReader(buffer.Bytes()),
			Key:             aws.String(key),
			ContentEncoding: aws.String("utf-8"),
			ContentType:     aws.String("plain/text")}

		_, err := f.s3.PutObject(&request)
		if err != nil {
			log.Fatal("failed to put object to bucket", err)
		}
		buffer.Reset()
	}
}

func newUUID() []byte {
	uuid := make([]byte, 16)
	n, err := io.ReadFull(rand.Reader, uuid)
	if n != len(uuid) || err != nil {
		log.Fatal("failed to read random generator.", err)
	}
	return uuid
}

type S3LogForwarder struct {
	channel chan []byte

	verbose       bool

	https         bool
	listenPort    int
	listenAddress string

	keyPrefix    string
	bucketRegion string
	bucketName   string

	flushPeriod time.Duration
	cacheSize   int

	s3 *s3.S3
}
