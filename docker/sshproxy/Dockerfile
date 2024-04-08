FROM golang:1.22.0 as builder

# Build frontend code which added upload/download button

ENV CGO_ENABLED 0
ENV GOOS linux

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
#RUN go mod download

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/ cmd/
COPY pkg/ pkg/

ARG SSHPIPER_VERSION=1.2.8

#https://github.com/tg123/sshpiper/releases/download/v1.2.8/sshpiperd_with_plugins_linux_arm64.tar.gz

RUN arch=$(uname -m | sed -E 's/aarch64/arm64/g') \
    && mkdir sshpiperTmp  \
    && cd sshpiperTmp  \
    && curl -LO https://github.com/tg123/sshpiper/releases/download/v$SSHPIPER_VERSION/sshpiperd_with_plugins_linux_${arch}.tar.gz \
    && tar -zxf sshpiperd_with_plugins_linux_${arch}.tar.gz \
    && cp sshpiperd /workspace \
    && cd - \
    && rm -rf sshpiperTmp

## Build
RUN cd /workspace && go build -ldflags="-s -w" -a -o sshproxy cmd/sshproxy/main.go
#
##
#FROM node:18.16.0
FROM gcr.io/distroless/base:nonroot

WORKDIR /

COPY --from=builder /workspace/sshproxy /bin/sshproxy
COPY --from=builder /workspace/sshpiperd /bin/sshpiperd


USER root
WORKDIR /root



