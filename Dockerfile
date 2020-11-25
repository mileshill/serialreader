FROM golang:1.15.5 AS build
WORKDIR /serial
COPY cmd /serial
RUN ls /serial
RUN go get ./cmd/ ...


RUN go build -o /bin/reader ./cmd/reader

