package main

import (
	"github.com/replicatedhq/troubleshoot/cmd/docsgen/cli"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func main() {
	cli.InitAndExecute()
}
