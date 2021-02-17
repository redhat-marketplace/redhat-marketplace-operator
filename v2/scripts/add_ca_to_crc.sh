#!/usr/bin/env bash

CERTDIR=${CERTDIR}
CA_CERT=${CA_CERT}
CRC_IP=`crc ip`

set -e

echo "Rsyncing our certs"
rsync -e "ssh -i $HOME/.crc/machines/crc/id_rsa -o IdentitiesOnly=yes" $CERTDIR/${CA_CERT} core@${CRC_IP}:/home/core

echo "Update CA"
ssh -i $HOME/.crc/machines/crc/id_rsa -p 22 core@${CRC_IP} -o IdentitiesOnly=yes "sudo rm -f /etc/pki/ca-trust/source/anchors/${CA_CERT} && sudo ln -s /home/core/${CA_CERT} /etc/pki/ca-trust/source/anchors/${CA_CERT}"
ssh -i $HOME/.crc/machines/crc/id_rsa -p 22 core@${CRC_IP} -o IdentitiesOnly=yes "sudo update-ca-trust extract"

echo "Verifying"
ssh -i $HOME/.crc/machines/crc/id_rsa -p 22 core@${CRC_IP} -o IdentitiesOnly=yes "sudo openssl crl2pkcs7 -nocrl -certfile /etc/pki/tls/certs/ca-bundle.crt | openssl pkcs7 -print_certs | grep subject | head"
