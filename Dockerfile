# build stage
FROM golang:alpine AS build-env
ADD . /go/src/forwarder
RUN env && \
	cd /go/src/forwarder && \
	go build -o forwarder


# final stage
FROM alpine
RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*
WORKDIR /app
ADD server.crt server.key /app/
COPY --from=build-env /go/src/forwarder/forwarder /app/
EXPOSE 4443
ENTRYPOINT /app/forwarder
