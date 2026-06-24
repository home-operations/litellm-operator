# Build the manager binary
ARG GO_VERSION
FROM golang:${GO_VERSION}-alpine AS builder
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG REVISION=dev

# upx (build stage only) compresses the final binary to shrink the image.
RUN apk add --no-cache upx

WORKDIR /workspace
# Download deps in their own layer so source changes don't invalidate the cache.
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY cmd/main.go cmd/main.go
COPY api/ api/
COPY internal/ internal/

# GOARCH is left unset so the binary matches the build host (or TARGETARCH under buildx).
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -a -ldflags "-X main.version=${VERSION} -X main.commit=${REVISION}" \
    -o manager cmd/main.go

RUN upx --best --lzma manager

# https://github.com/GoogleContainerTools/distroless
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .

ENTRYPOINT ["/manager"]
