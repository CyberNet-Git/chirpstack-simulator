FROM golang:1.18-alpine AS build

ARG VERSION=dev
ARG GO_EXTRA_BUILD_ARGS="-a -installsuffix cgo"

ENV CGO_ENABLED=0

WORKDIR /src

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build ${GO_EXTRA_BUILD_ARGS} \
  -ldflags "-s -w -X main.version=${VERSION}" \
  -o /out/chirpstack-simulator \
  cmd/chirpstack-simulator/main.go

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

COPY --from=build /out/chirpstack-simulator /usr/local/bin/chirpstack-simulator

ENTRYPOINT ["chirpstack-simulator"]
