FROM registry.access.redhat.com/ubi9/ubi-minimal:latest
ARG TARGETPLATFORM
ARG TARGETARCH
ARG TARGETOS
ARG quay_expiration=7d

COPY remediate.sh /usr/local/bin/remediate.sh

# create /etc/tmpfiles.d/rootfiles.conf to remdiate xccdf_org.ssgproject.content_rule_rootfiles_configured
# discard remediate requests to install packages with dnf
# find & xargs are required for some remediations
RUN mkdir -p /etc/tmpfiles.d && \
    cp /usr/lib/tmpfiles.d/rootfiles.conf /etc/tmpfiles.d/rootfiles.conf && \
    alias mycommand='true # ' && \
    microdnf install -y findutils crypto-policies-scripts && \
    /usr/local/bin/remediate.sh && \
    microdnf remove -y findutils crypto-policies-scripts
