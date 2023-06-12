package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func Collect(cmd *cobra.Command, args []string) {
		fmt.Println("collecting with specs at:",strings.Join(args,","),"...")
}