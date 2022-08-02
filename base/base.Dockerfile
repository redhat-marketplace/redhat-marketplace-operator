FROM registry.access.redhat.com/ubi8/ubi:latest as golang
ARG TARGETPLATFORM
ARG TARGETARCH
ARG TARGETOS
ARG GO_VERSION
ARG ARCH=${TARGETARCH}
ARG OS=${TARGETOS:-linux}
ENV VERSION=${GO_VERSION} OS=${OS} ARCH=${ARCH}

RUN dnf -y install git make yum gzip jq \
  && dnf -y update \
  && yum -y update-minimal --security --sec-severity=Important --sec-severity=Critical \
  && yum -y clean all \
  && dnf -y clean all \
  && rm -rf /var/cache/yum


RUN FOUND_VER=$(curl -s 'https://go.dev/dl/?mode=json' -H 'Accept: application/json' | jq -r '.[].version|select(contains(env.VERSION))') && \
  echo "go major version: $VERSION, found latest stable minor version: $FOUND_VER" && \
  curl -L -o go.tar.gz https://go.dev/dl/$FOUND_VER.$OS-$ARCH.tar.gz && \
  rm -rf /usr/local/go && \
  sha256sum go.tar.gz && \
  tar -C /usr/local -xzf go.tar.gz && \
  echo 'PATH=$PATH:/usr/local/go/bin' >> /etc/profile && \
  echo 'PATH=$PATH:/usr/local/go/bin' >> $HOME/.profile \
  echo 'PATH=$PATH:/usr/local/go/bin' >> $HOME/.profile
