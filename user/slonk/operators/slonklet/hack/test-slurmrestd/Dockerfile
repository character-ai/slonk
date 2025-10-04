FROM golang:latest

COPY . /slonklet
WORKDIR /slonklet
RUN go build -o /slonklet/bin/test-slurmrestd /slonklet/hack/test-slurmrestd/main.go

CMD ["/slonklet/bin/test-slurmrestd"]
