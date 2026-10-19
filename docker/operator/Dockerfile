# Build the manager binary
FROM golang:1.24.13 as builder

ENV CGO_ENABLED 0
ENV GOOS linux

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/ cmd/
COPY pkg/ pkg/

# Build
RUN go build -ldflags="-s -w" -a -o manager cmd/main.go && echo "Building GOARCH of $GOARCH.."

# The CGO-disabled manager needs no shared libraries, including OpenSSL.
# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=builder /workspace/manager /bin/manager
USER 65532:65532
