#!/usr/bin/env bash

set -euo pipefail

if [ -n "${COSIGN_KEY}" ]
then 
	echo "Writing cosign key to file"
	echo "${COSIGN_KEY}" | base64 -d > ./cosign.key
else 
	echo "ERROR: Missing COSIGN_KEY!"
fi

if ! command -v spdx-sbom-generator &> /dev/null
then
	echo "Installing spdx-sbom-generator"
        curl -L https://github.com/spdx/spdx-sbom-generator/releases/download/v0.0.13/spdx-sbom-generator-v0.0.13-linux-amd64.tar.gz -o ./sbom/spdx-sbom-generator.tar.gz
        curl -L https://github.com/spdx/spdx-sbom-generator/releases/download/v0.0.13/spdx-sbom-generator-v0.0.13-linux-amd64.tar.gz.md5 -o ./sbom/spdx-sbom-generator.tar.gz.md5
        md5sum ./sbom/spdx-sbom-generator.tar.gz | cut --bytes=1-32 > ./sbom/checksum

        if ! cmp ./sbom/checksum ./sbom/spdx-sbom-generator.tar.gz.md5
	then
        	echo "ERROR: spdx-sbom-generator.tar.gz md5 sum does not match!"
		exit 1
	fi

	tar -xzvf ./sbom/spdx-sbom-generator.tar.gz -C sbom
fi
