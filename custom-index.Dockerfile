FROM quay.io/operator-framework/upstream-opm-builder:latest
LABEL operators.operatorframework.io.index.database.v1=/database/index.db
RUN chgrp -R 0 /etc && \
    chmod -R g+rwx /etc
ADD database/index.db /database/index.db
EXPOSE 50051
ENTRYPOINT ["/bin/opm"]
CMD ["registry", "serve", "--database", "/database/index.db"]
