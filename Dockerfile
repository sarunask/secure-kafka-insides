FROM golang:1.10.1-alpine

RUN apk update && apk upgrade && apk add --no-cache bash git

ENV SOURCES /go/src/github.com/sarunask/secure-kafka-insides
COPY . ${SOURCES}

RUN cd ${SOURCES} && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w"

RUN cp -r ${SOURCES}/templates ./templates

CMD ["/go/src/github.com/sarunask/secure-kafka-insides/secure-kafka-insides"]
EXPOSE 8080