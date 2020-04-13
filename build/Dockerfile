FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

ENV USER_UID=1001 \
    USER_NAME=redhat-marketplace-operator \
    ASSETS=/usr/local/bin/assets
# install operator binary
COPY build/_output/bin /usr/local/bin
COPY build/_output/assets /usr/local/bin/assets
COPY build/bin/entrypoint /usr/local/bin/entrypoint
COPY build/bin/user_setup /usr/local/bin/user_setup
RUN  /usr/local/bin/user_setup

WORKDIR /usr/local/bin
ENTRYPOINT ["/usr/local/bin/entrypoint"]
CMD ["redhat-marketplace-operator"]

USER ${USER_UID}
