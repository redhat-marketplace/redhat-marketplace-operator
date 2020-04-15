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
COPY build/_output/bin /usr/local/bin
COPY build/_output/assets /usr/local/bin/assets
COPY build/bin/entrypoint /usr/local/bin/entrypoint
COPY build/bin/user_setup /usr/local/bin/user_setup
COPY LICENSE  /licenses/
RUN  /usr/local/bin/user_setup

WORKDIR /usr/local/bin
ENTRYPOINT ["/usr/local/bin/entrypoint"]
CMD ["redhat-marketplace-operator"]

USER ${USER_UID}
