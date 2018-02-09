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
)

var channel chan []byte

func newUUID() string {
	uuid := make([]byte, 16)
	n, err := io.ReadFull(rand.Reader, uuid)
	if n != len(uuid) || err != nil {
		log.Fatalf("failed to read random generator: ", err)
	}
	uuid[8] = uuid[8]&^0xc0 | 0x80
	uuid[6] = uuid[6]&^0xf0 | 0x40
	return fmt.Sprintf("%x-%x-%x-%x-%x", uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:])
}

func putObject(s3Service *s3.S3, bucketName *string, buffer *bytes.Buffer) {
	if buffer.Len() > 0 {
		now := time.Now()
		uuid := newUUID()
		hostName, _ := os.Hostname()
		key := fmt.Sprintf("/%04d/%02d/%02d/%04d%02d%02dT%02d%02d%02d.%06dZ-%s-%s.log",
			now.Year(),now.Month(), now.Day(), now.Year(),now.Month(), now.Day(),
			now.Hour(), now.Minute(), now.Second(), now.Nanosecond(), hostName, uuid)
		fmt.Printf("writing buffer of %d bytes to %s\n", buffer.Len(), key)
		request := s3.PutObjectInput{
			Bucket:          bucketName,
			Body:			 bytes.NewReader(buffer.Bytes()),
			Key:             aws.String(key),
			ContentEncoding: aws.String("utf-8"),
			ContentType:     aws.String("plain/text")}

		_, err := s3Service.PutObject(&request)
		if err != nil {
			log.Fatalf("failed to put object to bucket: ", err)
		}
		buffer.Reset()
	}
}

func bufferedPut(s3Service *s3.S3, bucketName *string) {
	var buffer bytes.Buffer
	timer := time.NewTimer(time.Second * 30)
	for {
		select {
		case body, more := <-channel:
			buffer.Write(body)
			if body[len(body)-1] != 10 {
				buffer.Write([]byte("\n"))
			}
			if !more || buffer.Len() > 4096 {
				putObject(s3Service, bucketName, &buffer)
			}
			if !more {
				return
			}
		case <-timer.C:
			putObject(s3Service, bucketName, &buffer)
			timer.Reset(time.Second * 30)
		}
	}
}

func HelloServer(w http.ResponseWriter, req *http.Request) {
	if req.Method == "POST" {
		body, err := ioutil.ReadAll(req.Body)
		if err == nil {
			channel <- body
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprintf(w, "%d bytes\n", len(body))
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	} else {
		http.Error(w, "unsupported HTTP method", http.StatusBadRequest)
	}
}

func setupS3Session(region *string) *s3.S3 {
	sess, err := session.NewSession()
	if err != nil {
		log.Fatal("sessionNewSession: ", err)
	}
	svc := s3.New(sess, &aws.Config{Region: aws.String(*region)})
	return svc
}

func main() {
	region := "eu-central-1"
	bucketName := "kong-api-gateway-logs"
	channel = make(chan []byte, 2048)
	go bufferedPut(setupS3Session(&region), &bucketName)
	http.HandleFunc("/", HelloServer)
	err := http.ListenAndServeTLS(":4443", "server.crt", "server.key", nil)
	close(channel)

	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
