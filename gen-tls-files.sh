#!/usr/bin/env bash
# Run this application to generate tls certificates i.e ./gen-tls-files.sh <dir-to-place-files>
# you need to install the CLI applications required by running the command below
#
#   go install github.com/cloudflare/cfssl/cmd/...@latest
#
# TODO: Remove me before merging PR. This is for testing purposes only

set -o nounset
set -o errexit

DIR=$1

function generateCA() {
  cat >ca-config.json <<EOF
{
  "signing": {
    "default": {
      "expiry": "8760h"
    },
    "profiles": {
      "redis": {
        "usages": ["signing", "key encipherment", "server auth", "client auth"],
        "expiry": "8760h"
      }
    }
  }
}
EOF

  cat >ca-csr.json <<EOF
{
  "CN": "Redis",
  "key": {
    "algo": "rsa",
    "size": 4096
  },
  "names": [
    {
      "C": "UK",
      "L": "Didcot",
      "O": "Redis",
      "OU": "CA",
      "ST": "Oxfordshire"
    }
  ]
}
EOF

  cfssl gencert -initca ca-csr.json | cfssljson -bare ca
}

function generateClient() {
  name=$1
  cat >${name}-csr.json <<EOF
{
  "CN": "${name}",
  "key": {
    "algo": "rsa",
    "size": 4096
  },
  "names": [
    {
      "C": "UK",
      "L": "Didcot",
      "O": "Redis",
      "OU": "CA",
      "ST": "Oxfordshire"
    }
  ]
}
EOF

  cfssl gencert \
    -ca=ca.pem \
    -ca-key=ca-key.pem \
    -config=ca-config.json \
    -hostname=${name} \
    -profile=redis \
    ${name}-csr.json | cfssljson -bare ${name}

}

FULLPATH=$(realpath $DIR)
mkdir -p $FULLPATH

pushd $DIR > /dev/null
generateCA
generateClient server
generateClient client
rm *.json *.csr
popd > /dev/null
echo -e "\n\n============== list generated files in '$FULLPATH' =============\n"
ls -al $FULLPATH
