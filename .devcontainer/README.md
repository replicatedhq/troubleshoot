# Replicated KOTS Codespace Container

Most of the code here is borrowed from this [Microsoft repo of base images](https://github.com/microsoft/vscode-dev-containers), except for replicated specific things.

## Notes
* k3d *DOES NOT* work with DinD. You have to use the docker with docker install instead.
* Might be faster to install kubectl plugins on the `$PATH` in the `Dockerfile` instead of downloading them `onCreate.sh`.
