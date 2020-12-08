FROM quay.io/rh-marketplace/golang-base:1.15 as base

WORKDIR /usr/local/victoria/build
ARG VICTORIA_VERSION=v1.48.0

RUN curl -o ./victora.tar.gz https://github.com/VictoriaMetrics/VictoriaMetrics/archive/${VICTORIA_VERSION}.tar.gz
RUN ls
RUN tar -xzf victoria.tar.gz
RUN make victoria-metrics-prod
