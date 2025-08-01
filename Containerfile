FROM docker.io/golang:1.23-alpine AS builder

RUN apk add make

ENV CGO_ENABLED=0
WORKDIR /src
COPY go.* Makefile ./
COPY v2 ./v2/
RUN go env -w GOCACHE=/cache/go-build GOMODCACHE=/cache/go-mod
RUN --mount=type=cache,target=/cache/go-build --mount=type=cache,target=/cache/go-mod make

FROM docker.io/alpine:latest

RUN apk add bash
COPY --from=builder /src/display1306 /usr/local/bin/
