package main

import (
	"github.com/joho/godotenv"
	"github.com/replicatedhq/troubleshoot/cmd/analyze/cli"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func main() {
	// Load .env file if it exists (ignore error if not found)
	_ = godotenv.Load()
	
	cli.InitAndExecute()
}
