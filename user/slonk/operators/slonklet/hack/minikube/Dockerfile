FROM golang:latest

COPY . /slonklet
WORKDIR /slonklet
RUN make build-controller

CMD ["/slonklet/bin/slonklet-controller"]
