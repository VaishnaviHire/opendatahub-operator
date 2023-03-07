# Build the manager binary
ARG GOLANG_VERSION=1.18.4
FROM registry.access.redhat.com/ubi8/go-toolset:$GOLANG_VERSION as builder

WORKDIR /workspace
USER root
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY apis/ apis/
COPY controllers/ controllers/
COPY pkg/ pkg/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build --trimpath -a -o build/_output/manager -ldflags="-X 'github.com/opendatahub-io/opendatahub-operator/pkg/kfapp/kustomize.enableKustAlphaPlugin=yes'" main.go

RUN cd kustomize-fns/v1alpha1/applyresources && go mod vendor && \
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build --trimpath --mod=vendor --buildmode=plugin -o build/_output/plugins/v1alpha1/applyresources/ApplyResources.so ApplyResources.go

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest
WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=builder /opt/build/_output/plugins/ $HOME/.config/kustomize/plugin/
USER 65532:65532

ENTRYPOINT ["/manager"]
