FROM --platform=${BUILDPLATFORM}  golang:1.16-alpine AS builder
ARG OPM_VERSION

RUN apk update && apk add sqlite build-base git mercurial bash
WORKDIR /build

ENV ARCHS="amd64 ppc64le s390x" \
    GOOS=linux

RUN git clone --branch v1.17.0 --depth 1 https://github.com/operator-framework/operator-registry.git .
RUN for arch in ${ARCHS}; do \
    GOFLAGS="-mod=vendor" GOOS=linux GOARCH=${arch} go build -ldflags '-w -extldflags "-static"' -tags "json1" -o bin/opm-linux-${arch} ./cmd/opm ; \
    done

FROM alpine
ARG TARGETARCH
ARG TARGETOS

RUN apk update && apk add ca-certificates
RUN GRPC_HEALTH_PROBE_VERSION=v0.3.2 && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-${TARGETARCH} && \
    chmod +x /bin/grpc_health_probe

COPY --from=builder ["/build/nsswitch.conf", "/etc/nsswitch.conf"]
COPY --from=builder /build/bin/opm-${TARGETOS}-${TARGETARCH} /bin/opm
