FROM registry.access.redhat.com/ubi8/ubi:latest as builder
ARG OPM_VERSION
ARG TARGETARCH
ARG TARGETOS

WORKDIR /build

RUN dnf -y install git make wget

RUN /bin/sh -c 'wget "https://mirror.openshift.com/pub/openshift-v4/$(uname -m)/clients/ocp/4.9.0/opm-linux.tar.gz" && \
    tar -xf opm-linux.tar.gz && \
    chmod +x ./opm'

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

COPY --from=builder ["/build/opm", "/bin/opm"]
