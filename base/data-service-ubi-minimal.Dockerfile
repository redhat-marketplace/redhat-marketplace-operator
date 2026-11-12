ARG REGISTRY=quay.io/rh-marketplace

FROM ${REGISTRY}/data-service-base:ubi9 as dqlite-lib-builder

FROM ${REGISTRY}/ubi-minimal:ubi9
ARG TARGETPLATFORM
ARG TARGETARCH
ARG TARGETOS
ARG quay_expiration=7d

RUN rpm -ivh https://dl.fedoraproject.org/pub/epel/epel-release-latest-9.noarch.rpm && \
    microdnf install -y libsqlite3x libuv && \
    microdnf clean all && \
    rpm -e epel-release-9

# COPY dqlite from builder
COPY --from=dqlite-lib-builder /usr/lib64/libdqlite.* /usr/lib64/
COPY --from=dqlite-lib-builder /usr/lib64/pkgconfig/dqlite.pc /usr/lib64/pkgconfig/dqlite.pc

