FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

ENV OPERATOR=/usr/local/bin/marketplace-operator \
    USER_UID=1001 \
    USER_NAME=marketplace-operator \
    ASSETS=/usr/local/bin/assets

# install operator binary
COPY build/_output/bin/marketplace-operator ${OPERATOR}

COPY build/_output/assets /usr/local/bin/assets
COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
