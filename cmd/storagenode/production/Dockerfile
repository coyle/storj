
# build
FROM golang:1.11-alpine as build-env

ENV GOOS=linux
ENV GOARM=6
ENV GOARCH=arm
ENV CGO_ENABLED=1

ADD . /go/src/storj.io/storj
WORKDIR /go/src/storj.io/storj/cmd/storagenode

# dependencies + binary
RUN apk add --no-cache git gcc musl-dev && unset GOPATH && go mod vendor && go build -a -installsuffix cgo -o storagenode . && mkdir config

# final stage
FROM alpine

EXPOSE 28967

ENV STORJ_KADEMLIA_BOOTSTRAP_ADDR="bootstrap.storj.io:8888"
ENV STORJ_METRICS_APP_SUFFIX="-alpha"
ENV STORJ_METRICS_INTERVAL="30m"
ENV STORJ_SERVER_USE_PEER_CA_WHITELIST="true"
ENV COMMAND="run"

WORKDIR /app
COPY --from=build-env /go/src/storj.io/storj/cmd/storagenode/storagenode /app/
COPY --from=build-env /go/src/storj.io/storj/cmd/storagenode/config /app/

ENTRYPOINT ./storagenode $COMMAND --config-dir="config"