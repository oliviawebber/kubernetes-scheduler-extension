FROM golang:1.13-alpine
RUN mkdir -p /go/thermal-aware-scheduler
WORKDIR /go/thermal-aware-scheduler
RUN go mod init example.com/thermalAwareScheduler
COPY go.mod .
COPY go.sum .
COPY ./src .
EXPOSE 80
EXPOSE 4321
RUN go build scheduler-extension.go
ENTRYPOINT ["scheduler-extension"]

