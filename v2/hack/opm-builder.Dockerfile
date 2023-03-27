FROM registry.access.redhat.com/ubi8/ubi:latest as builder
ARG OPM_VERSION
ARG TARGETARCH
ARG TARGETOS

WORKDIR /build

RUN dnf -y install git make wget

RUN GRPC_HEALTH_PROBE_VERSION=v0.4.17 && \
    wget -qO./grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-${TARGETARCH} && \
    chmod +x ./grpc_health_probe

RUN /bin/sh -c 'wget "https://mirror.openshift.com/pub/openshift-v4/$(uname -m)/clients/ocp/4.9.0/opm-linux.tar.gz" && \
    tar -xf opm-linux.tar.gz && \
    chmod +x ./opm'

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

COPY --from=builder ["/build/grpc_health_probe", "/bin/grpc_health_probe"]
COPY --from=builder ["/build/opm", "/bin/opm"]
