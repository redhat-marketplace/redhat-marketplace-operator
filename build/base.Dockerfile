FROM registry.access.redhat.com/ubi8/ubi:latest as golang

ARG ARCH=amd64
ENV VERSION=1.15.1 OS=linux ARCH=${ARCH}

RUN dnf -y install git

RUN curl -o go$VERSION.$OS-$ARCH.tar.gz https://dl.google.com/go/go$VERSION.$OS-$ARCH.tar.gz && \
  tar -C /usr/local -xzf go$VERSION.$OS-$ARCH.tar.gz && \
  echo 'PATH=$PATH:/usr/local/go/bin' >> /etc/profile && \
  echo 'PATH=$PATH:/usr/local/go/bin' >> $HOME/.profile

RUN dnf update --setopt=tsflags=nodocs -y \
    && dnf clean all \
    && rm -rf /var/cache/yum
