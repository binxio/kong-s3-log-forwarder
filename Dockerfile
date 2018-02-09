# build stage
FROM golang:alpine AS build-env
ADD . /go/src/adapter
RUN env && \
	cd /go/src/adapter && \
	go build -o adapter


# final stage
FROM alpine
WORKDIR /app
ADD server.crt server.key /app/
COPY --from=build-env /go/src/adapter/adapter /app/
EXPOSE 4443
ENTRYPOINT /app/adapter
