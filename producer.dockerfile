FROM golang:1.15.5 AS build

# Install dependencies
COPY . /go/src/github.com/mileshill/serialreader
WORKDIR /go/src/github.com/mileshill/serialreader
RUN go mod download

RUN go install github.com/mileshill/serialreader/cmd/producer
ENTRYPOINT /go/bin/producer

