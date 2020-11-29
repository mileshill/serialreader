FROM golang:1.15.5 AS builder
# Git
RUN apt update && apt install -y git

# Project coopy
WORKDIR /go/src/github.com/mileshill/serialreader
COPY . /go/src/github.com/mileshill/serialreader

# Dependencies
RUN go mod download
#RUN go install github.com/mileshill/serialreader/cmd/producer
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /go/bin/reader ./cmd/reader

# Small image
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /go/bin
COPY --from=builder /go/bin/reader ./reader
ENTRYPOINT ["./reader"]

