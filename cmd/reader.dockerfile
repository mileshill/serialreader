FROM golang:1.15.5 AS build

# Install dependencies
RUN go get github.com/jacobsa/go-serial/serial
RUN go get -v -u go.mongodb.org/mongo-driver; exit 0

# Add code
WORKDIR /go/src/serialreader
COPY reader reader
COPY util util

CMD ["go", "run", "/go/src/serialreader/reader/main.go"]

