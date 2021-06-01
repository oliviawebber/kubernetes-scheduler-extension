FROM golang:1.13-alpine

COPY go.mod .
COPY go.sum .
COPY ./src .
EXPOSE 80
EXPOSE 4321
RUN go run scheduler-extension.go

