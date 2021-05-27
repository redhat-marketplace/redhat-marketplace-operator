#!/bin/bash
oc delete secret rhm-dqlite-mtls
oc create secret generic rhm-dqlite-mtls --from-file=ca.crt=ca.pem --from-file=tls.crt=server-cert.pem --from-file=tls.key=server-key.pem
