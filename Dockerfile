FROM library/golang:1.15.6 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN  go mod download
COPY Makefile VERSION .promu.yml .git ./
COPY iptables ./iptables
COPY iptables_exporter.go ./
RUN  make

FROM library/alpine:3.13
RUN  apk add --no-cache iptables
COPY --from=builder /src/iptables_exporter /bin/iptables-exporter
ENTRYPOINT ["iptables-exporter"]
