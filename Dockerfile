#1
FROM golang:latest AS compiling_stage

RUN mkdir -p /go/src/pipeline

WORKDIR /go/src/pipeline

ADD main.go .

ADD go.mod .

RUN go build -o pipeline .


#2
FROM alpine:latest

LABEL version="1.0.0"

LABEL maintainer="nikkfame<nikkfame@gmail.com"

WORKDIR /app

COPY --from=compiling_stage /go/src/pipeline/pipeline .

ENTRYPOINT ./pipeline