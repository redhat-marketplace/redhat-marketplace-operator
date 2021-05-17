#!/bin/bash

# Example of generating certificates for use with signer

set -euo pipefail

rm -Rf ca-key.pem ca.pem server-key.pem server-req.pem server-cert.pem out.yaml

openssl genrsa 2048 > ca-key.pem

openssl req -new -x509 -nodes -days 365000 -key ca-key.pem -out ca.pem -subj "/C=US/ST=NC/L=Raleigh/O=IBM/OU=RHM/"

openssl req -newkey rsa:2048 -days 365000 -nodes -keyout server-key.pem -out server-req.pem -subj "/C=US/ST=NC/L=Raleigh/O=IBM/OU=RHM/"

openssl rsa -in server-key.pem -out server-key.pem

openssl x509 -req -in server-req.pem -days 365000 -CA ca.pem -CAkey ca-key.pem -set_serial 01 -out server-cert.pem

openssl verify -CAfile ca.pem server-cert.pem