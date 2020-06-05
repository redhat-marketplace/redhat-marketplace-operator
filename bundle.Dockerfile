FROM scratch

LABEL operators.operatorframework.io.bundle.mediatype.v1=registry+v1
LABEL operators.operatorframework.io.bundle.manifests.v1=manifests/
LABEL operators.operatorframework.io.bundle.metadata.v1=metadata/
LABEL operators.operatorframework.io.bundle.package.v1=redhat-marketplace-operator-test
LABEL operators.operatorframework.io.bundle.channels.v1=stable,beta
LABEL operators.operatorframework.io.bundle.channel.default.v1=stable

COPY deploy/olm-catalog/redhat-marketplace-operator/0.1.2 /manifests/
COPY deploy/olm-catalog/redhat-marketplace-operator/metadata /metadata/
