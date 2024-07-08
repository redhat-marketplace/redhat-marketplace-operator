ARG REGISTRY=quay.io/rh-marketplace

FROM ${REGISTRY}/data-service-base:ubi8 as dqlite-lib-builder

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest
ARG TARGETPLATFORM
ARG TARGETARCH
ARG TARGETOS
ARG quay_expiration=7d

# microdnf can not install from url, must curl & rpm
RUN curl -sSL -o epel-release-latest-8.noarch.rpm https://dl.fedoraproject.org/pub/epel/epel-release-latest-8.noarch.rpm && \
    rpm -i epel-release-latest-8.noarch.rpm && \
    microdnf install libsqlite3x libuv && \
    microdnf clean all && \
    rpm -e epel-release-8 && \
    rm -Rf epel-release-latest-8.noarch.rpm

# COPY dqlite from builder
COPY --from=dqlite-lib-builder /usr/lib64/libdqlite.* /usr/lib64/
COPY --from=dqlite-lib-builder /usr/lib64/pkgconfig/dqlite.pc /usr/lib64/pkgconfig/dqlite.pc

