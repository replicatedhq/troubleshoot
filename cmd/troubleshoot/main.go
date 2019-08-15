package main

import (
	"github.com/replicatedhq/troubleshoot/cmd/troubleshoot/cli"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func main() {
	cli.InitAndExecute()
}
