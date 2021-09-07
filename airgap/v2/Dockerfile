FROM ubuntu as dqlite-lib-builder
ARG DEBIAN_FRONTEND="noninteractive"
ARG ARCH=amd64
ENV TZ=America/New_York
ENV LD_LIBRARY_PATH=/usr/local/lib
ENV GOROOT=/usr/local/go
ENV GOPATH=/go
ENV PATH=$GOPATH/bin:$GOROOT/bin:$PATH
ENV GO_VERSION=1.16 OS=linux ARCH=${ARCH}

RUN apt-get update && apt-get install -y git build-essential dh-autoreconf pkg-config libuv1-dev libsqlite3-dev tcl8.6 wget liblz4-1 liblz4-dev

WORKDIR /opt

RUN git clone --depth 1 https://github.com/canonical/raft.git && \
    git clone --depth 1 https://github.com/canonical/dqlite.git && \
    wget -c https://dl.google.com/go/go$GO_VERSION.$OS-$ARCH.tar.gz -O - | tar -xzf - -C /usr/local

WORKDIR /opt/raft

RUN autoreconf -i && ./configure && make && make install

WORKDIR /opt/dqlite

RUN autoreconf -i && ./configure && make && make install

WORKDIR /opt/airgap

COPY airgap/v2 .

RUN go get -d -v ./... && \
    go install -tags libsqlite3 ./cmd/server/main.go

# FROM debian:buster-slim
FROM ubuntu
ARG DEBIAN_FRONTEND="noninteractive"
ENV TZ=America/New_York
ENV LD_LIBRARY_PATH=/usr/local/lib
ENV PATH=/opt:$PATH

COPY --from=dqlite-lib-builder /go/bin /opt/
COPY --from=dqlite-lib-builder /usr/local/lib /usr/local/lib
COPY --from=dqlite-lib-builder \
    /usr/lib/x86_64-linux-gnu/libuv.so \
    /usr/lib/x86_64-linux-gnu/libuv.so.1 \
    /usr/lib/x86_64-linux-gnu/libuv.so.1.0.0 \
    /usr/lib/

COPY --from=dqlite-lib-builder \
    /lib/x86_64-linux-gnu/libsqlite3.so \
    /lib/x86_64-linux-gnu/libsqlite3.so.0 \
    /usr/lib/x86_64-linux-gnu/
