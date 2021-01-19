#!/usr/bin/env bash

# Arguments
IN_CERT_DIR=${CERTDIR:-'.'}
IN_DOMAIN=${DOMAIN:-'*.apps-crc.testing'}
IN_NAME=${NAME-'registry'}

# Variables
IN_CA_KEY=${IN_NAME}-rootCA.key
IN_CA_CERT=${IN_NAME}-rootCA.crt
IN_CA_CSR="${DOMAIN}.csr"
IN_CA_CSR_KEY="${DOMAIN}.key"
IN_CERT_NAME=${IN_NAME}-cert.crt

if ! [ -x "$(command -v openssl)" ]; then
    echo 'Error: openssl command needed. Try brew install openssl.' >&2
    exit 1
fi

cd $IN_CERT_DIR

array=("${IN_DOMAIN}")
echo ""
echo "------ Starting creating cert -------"
echo "Making cert with domains = ${array[@]}"

DOMAINS=""
len=${#array[@]}

for (( i=0; i<$len; i++ ))
do
  elem="${array[$i]}"
  DOMAINS="$DOMAINS\nDNS.$i=$elem"
done

DOMAIN=${array[0]}

# CA

echo "Generating CA Key"
openssl genrsa -out ${IN_CA_KEY} 4096
echo "Generating CA"
openssl req -x509 -new -nodes -sha256 \
  -key ${IN_CA_KEY} \
  -days 90 \
  -out ${IN_CA_CERT} \
  -subj "/C=US/ST=North Carolina/L=Durham/O=IBM, Inc./OU=IT/CN=${DOMAIN}"

# CSR

echo "Generating Cert Key"
openssl genrsa -out ${IN_CA_CSR_KEY} 2048
echo "Generating signing (CSR)"
openssl req -new -sha256 \
  -key ${IN_CA_CSR_KEY} \
  -subj "/C=US/ST=NC/O=IBM, Inc./CN=${DOMAIN}" \
  -out ${IN_CA_CSR} \
  -reqexts SAN \
  -config <(cat /etc/ssl/openssl.cnf \
            <(printf "\n[SAN]\nsubjectAltName=DNS:$DOMAIN"))

echo "Verifying CSR"
openssl req -in ${IN_CA_CSR} -noout -text

# Cert

echo "Generating cert"
openssl x509 -req \
  -in $IN_CA_CSR \
  -CA ${IN_CA_CERT} \
  -CAkey ${IN_CA_KEY} \
  -CAcreateserial \
  -out $IN_CERT_NAME  \
  -sha256 \
  -days 90

echo ""
echo "------ Finished creating cert -------"
