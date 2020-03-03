#!/usr/bin/env bash

INCERTDIR=${CERTDIR:-'.'}

if ! [ -x "$(command -v openssl)" ]; then
    echo 'Error: openssl command needed. Try brew install openssl.' >&2
    exit 1
fi

array=("*.apps-crc.testing")
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

INCERTNAME=${INCERTDIR}/${CERTNAME:-registry-crc.crt}
INKEYNAME=${INCERTDIR}/${KEYNAME:-registry-crc.key}
openssl req -x509 -out $INCERTNAME -keyout $INKEYNAME \
        -newkey rsa:4096 -nodes -sha256 -days 90 \
        -subj "/CN=${DOMAIN}" -extensions EXT -config <( printf "[dn]\nCN=${DOMAIN}\n[req]\ndistinguished_name = dn\n[EXT]\nsubjectAltName = @alt_names\nkeyUsage=digitalSignature\nextendedKeyUsage=serverAuth\n[alt_names]$DOMAINS")
echo ""
echo "Adding cert to your local files. Enter your password."
sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain -p ssl -p basic $INCERTNAME

echo ""
echo "------ Finished creating cert -------"
