# Build the manager binary
FROM golang:1.22 as builder

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.sum ./
# cache deps before building and copying source so that we don't need to re-build as much
# and so that source changes don't invalidate our built layer
COPY vendor/ vendor/

# Copy the project source
COPY api/ api/
COPY controllers/ controllers/
COPY webhook/ webhook/
COPY pkg/ pkg/
COPY config/ config/
COPY metrics/ metrics/
COPY console/ console/
COPY hack/ hack/
COPY main.go Makefile ./

# Run tests and linting
RUN make go-test

# Build
RUN make go-build

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/bin/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
