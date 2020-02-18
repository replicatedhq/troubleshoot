# Development Environment Setup

1. Ensure that you have `go` installed
2. Ensure that your PATH is set to include the GOPATH in your `.bashrc` file. For example: 
```
export GOPATH=/home/username/go
export PATH="${KREW_ROOT:-$HOME/.krew}/bin:$GOPATH:$PATH"
```

# Testing Troubleshoot Locally

1. Run `make support-bundle` and `make ffi`
2. Copy `./bin/support-bundle` to `kotsamd/operator` folder
3. In docker.skaffold, uncomment # COPY ./troubleshoot.so /lib/troubleshoot.so
4. In docker.skaffold, uncomment # COPY ./support-bundle /root/.krew/bin/kubectl-support_bundle

After these steps, the operator will restart (enabling local verification). 
