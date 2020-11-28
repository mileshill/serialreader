FROM golang:1.15.5 AS build

# Install dependencies
COPY . /go/src/github.com/mileshill/serialreader
WORKDIR /go/src/github.com/mileshill/serialreader
RUN go mod download

RUN go install github.com/mileshill/serialreader/cmd/reader
ENTRYPOINT /go/bin/reader

