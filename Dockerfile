FROM golang:1

WORKDIR /go/src/app
COPY . .

RUN go get -d -v ./...
RUN go install -v ./...
RUN go build -o ./ecobee_influx_connector .

CMD /go/src/app/ecobee_influx_connector -config "/config/config.json"

VOLUME /config