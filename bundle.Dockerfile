FROM scratch
ARG channels=stable,beta
ARG default-channel=stable
ARG manifests=deploy/olm-catalog/redhat-marketplace-operator/manifests

LABEL operators.operatorframework.io.bundle.mediatype.v1=registry+v1
LABEL operators.operatorframework.io.bundle.manifests.v1=manifests/
LABEL operators.operatorframework.io.bundle.metadata.v1=metadata/
LABEL operators.operatorframework.io.bundle.package.v1=redhat-marketplace-operator
LABEL operators.operatorframework.io.bundle.channels.v1=${channels}
LABEL operators.operatorframework.io.bundle.channel.default.v1=${default-channel}
LABEL operators.operatorframework.io.metrics.mediatype.v1=metrics+v1
LABEL operators.operatorframework.io.metrics.builder=operator-sdk-v0.19.4
LABEL operators.operatorframework.io.metrics.project_layout=go

COPY ${manifests} /manifests/
COPY deploy/olm-catalog/redhat-marketplace-operator/metadata /metadata/
