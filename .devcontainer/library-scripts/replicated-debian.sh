#!/usr/bin/env bash

# k3d
# v5 RC is needed to deterministically set the Registry port. Should be replaces with official release
curl -s https://raw.githubusercontent.com/rancher/k3d/main/install.sh | TAG=v5.0.0-rc.4 bash

# kustomize
pushd /tmp
curl -s "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"  | bash
popd
sudo mv /tmp/kustomize /usr/local/bin/ 


