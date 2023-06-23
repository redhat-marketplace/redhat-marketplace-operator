ARG REGISTRY=quay.io/rh-marketplace
ARG OPM_VERSION

FROM ${REGISTRY}/opm-base:${OPM_VERSION}
LABEL operators.operatorframework.io.index.database.v1=/database/index.db
ADD database/index.db /database/index.db
EXPOSE 50051
ENTRYPOINT ["/bin/opm"]
CMD ["registry", "serve", "--database", "/database/index.db"]
