FROM golang:1.15-alpine as base


ENV PATH=$PATH:/usr/local/go/bin CGO_ENABLED=0 GOOS=linux

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

FROM golang:1.15-alpine as builder

WORKDIR /src

RUN --mount=target=. \
   --mount=type=cache,target=/go/pkg/mod \
   --mount=type=cache,target=/root/.cache/go-build \
   go build \
   -gcflags=-trimpath=$GOPATH -asmflags=-trimpath=$GOPATH \
   -o /app/bin/mockauth ./test/mockcontrollers

FROM alpine:3.12

ARG USER=default
ENV HOME /home/$USER

# install sudo as root
RUN apk add --update sudo

# add new user
RUN adduser -D $USER \
        && echo "$USER ALL=(ALL) NOPASSWD: ALL" > /etc/sudoers.d/$USER \
        && chmod 0440 /etc/sudoers.d/$USER

USER $USER
WORKDIR $HOME

COPY --chown=${USER} test/certs/* $HOME/
COPY --chown=${USER} --from=builder /app/bin/mockauth $HOME

CMD cd $HOME && ./mockauth --namespace openshift-redhat-marketplace \
  --server-cert $HOME/server.pem \
  --server-key $HOME/server-key.pem \
  --ca-cert $HOME/ca.crt
