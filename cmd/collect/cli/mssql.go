package cli

import (
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/spf13/cobra"
)

// MssqlCmd runs the mssql collector against a single database and prints the
// native result JSON ({"isConnected":..,"version":..,"error":..}).
//
// Unlike the other database collectors, the mssql collector does not honor a
// separate TLS field — TLS is configured through the connection URI query
// parameters (e.g. encrypt=true) — so this subcommand only takes --uri.
func MssqlCmd() *cobra.Command {
	var uri string

	cmd := &cobra.Command{
		Use:   "mssql",
		Short: "Run the mssql collector against a database",
		Long: `Run the mssql collector: connect to a Microsoft SQL Server, run "select @@VERSION",
and print the collector result as JSON.

--uri is a go-mssqldb connection URL; TLS options are passed as query parameters:
  sqlserver://user:password@host:1433?database=app&encrypt=true

Example:
  collect mssql --uri "sqlserver://sa:pass@mssql.default.svc:1433?encrypt=true"`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			db := &troubleshootv1beta2.Database{
				CollectorMeta: troubleshootv1beta2.CollectorMeta{CollectorName: "mssql"},
				URI:           uri,
			}

			c := &collect.CollectMssql{Collector: db, Context: cmd.Context()}
			res, err := c.Collect(nil)
			if err != nil {
				return err
			}
			return printCollectorResult(res)
		},
	}

	cmd.Flags().StringVar(&uri, "uri", "", "SQL Server connection URI, e.g. sqlserver://user:pass@host:1433 (required)")
	cmd.MarkFlagRequired("uri")

	return cmd
}
