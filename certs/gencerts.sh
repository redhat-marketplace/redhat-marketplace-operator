#!/bin/bash

openssl genrsa 2048 > ca-key.pem

openssl req -new -x509 -nodes -days 365000 -key ca-key.pem -out ca.pem \
  -subj "/C=US/ST=NC/O=IBM./CN=127.0.0.1.ca" 

openssl req -newkey rsa:2048 -days 365000 -nodes -keyout server-key.pem -out server-req.pem \
  -subj "/C=US/ST=NC/O=IBM./CN=127.0.0.1" \
  -extensions v3_req \
  -reqexts SAN \
  -config <(cat /etc/pki/tls/openssl.cnf \
        <(printf "\n[SAN]\nsubjectAltName=DNS.1:*.svc.cluster.local,IP.1:127.0.0.1"))


openssl rsa -in server-key.pem -out server-key.pem

openssl x509 -req -in server-req.pem -days 365000 -CA ca.pem -CAkey ca-key.pem -set_serial 01 -out server-cert.pem \
  -extensions SAN \
  -extfile <(cat /etc/pki/tls/openssl.cnf \
        <(printf "\n[SAN]\nsubjectAltName=DNS.1:*.svc.cluster.local,IP.1:127.0.0.1"))

openssl verify -CAfile ca.pem server-cert.pem