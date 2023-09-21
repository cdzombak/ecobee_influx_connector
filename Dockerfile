ARG BIN_NAME=ecobee_influx_connector
ARG BIN_VERSION=<unknown>

FROM golang:1-alpine AS builder
ARG BIN_NAME
ARG BIN_VERSION
WORKDIR /src/ecobee_influx_connector
RUN apk add --no-cache ca-certificates
COPY . .
RUN go build -ldflags="-X main.version=${BIN_VERSION}" -o ./out/${BIN_NAME} .

FROM scratch
ARG BIN_NAME
COPY --from=builder /src/ecobee_influx_connector/out/${BIN_NAME} /usr/bin/ecobee_influx_connector
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
VOLUME /config
ENTRYPOINT ["/usr/bin/ecobee_influx_connector"]
CMD ["-config", "/config/config.json"]

LABEL license="Apache-2.0"
LABEL maintainer="Chris Dzombak <https://www.dzombak.com>"
LABEL org.opencontainers.image.authors="Chris Dzombak <https://www.dzombak.com>"
LABEL org.opencontainers.image.url="https://github.com/cdzombak/ecobee_influx_connector"
LABEL org.opencontainers.image.documentation="https://github.com/cdzombak/ecobee_influx_connector/blob/main/README.md"
LABEL org.opencontainers.image.source="https://github.com/cdzombak/ecobee_influx_connector.git"
LABEL org.opencontainers.image.version="${BIN_VERSION}"
LABEL org.opencontainers.image.licenses="Apache-2.0"
LABEL org.opencontainers.image.title="${BIN_NAME}"
LABEL org.opencontainers.image.description="Ship your Ecobee runtime, sensor and weather data to InfluxDB."
