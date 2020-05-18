FROM registry.access.redhat.com/ubi8/ubi:latest as golang

ENV VERSION=1.14.2 OS=linux ARCH=amd64

RUN dnf -y install git

RUN curl -o go$VERSION.$OS-$ARCH.tar.gz https://dl.google.com/go/go$VERSION.$OS-$ARCH.tar.gz && \
  tar -C /usr/local -xzf go$VERSION.$OS-$ARCH.tar.gz && \
  echo 'PATH=$PATH:/usr/local/go/bin' >> /etc/profile && \
  echo 'PATH=$PATH:/usr/local/go/bin' >> $HOME/.profile

FROM golang as builder

ENV PATH=$PATH:/usr/local/go/bin

WORKDIR /usr/local/go/src/github.com/redhat-marketplace/redhat-marketplace-operator
COPY go.sum go.mod ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -o build/_output/bin/redhat-marketplace-operator ./cmd/manager

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

LABEL name="Red Hat Marketplace Operator" \
  maintainer="ztaylor@ibm.com" \
  vendor="Red Hat Marketplace" \
  release="1" \
  summary="Red Hat Marketplace Operator Image" \
  description="Operator for the Red Hat Marketplace"

ENV USER_UID=1001 \
    USER_NAME=redhat-marketplace-operator \
    ASSETS=/usr/local/bin/assets
# install operator binary
COPY --from=builder /usr/local/go/src/github.com/redhat-marketplace/redhat-marketplace-operator/build/_output/bin /usr/local/bin
COPY assets /usr/local/bin/assets
COPY build/bin/entrypoint /usr/local/bin/entrypoint
COPY build/bin/user_setup /usr/local/bin/user_setup
COPY LICENSE  /licenses/
RUN  /usr/local/bin/user_setup

WORKDIR /usr/local/bin
ENTRYPOINT ["/usr/local/bin/entrypoint"]
CMD ["redhat-marketplace-operator"]

USER ${USER_UID}
