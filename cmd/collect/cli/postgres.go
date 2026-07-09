package cli

import (
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// PostgresCmd runs the postgres collector against a single database and prints
// the native result JSON ({"isConnected":..,"version":..,"error":..}).
func PostgresCmd() *cobra.Command {
	var (
		uri             string
		skipVerify      bool
		caCert          string
		clientCert      string
		clientKey       string
		secretName      string
		secretNamespace string
	)

	cmd := &cobra.Command{
		Use:   "postgres",
		Short: "Run the postgres collector against a database",
		Long: `Run the postgres collector: connect to a PostgreSQL server, run "select version()",
and print the collector result as JSON.

--uri is a libpq/pgx connection URI:
  postgres://user:password@host:5432/dbname?sslmode=disable

TLS material may be provided inline / by file path (--tls-cacert, --tls-client-cert,
--tls-client-key) or sourced from a Secret (--tls-secret-name), which requires
cluster access.

Example:
  collect postgres --uri "postgres://user:pass@pg.default.svc:5432/app?sslmode=disable"`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			db := &troubleshootv1beta2.Database{
				CollectorMeta: troubleshootv1beta2.CollectorMeta{CollectorName: "postgres"},
				URI:           uri,
			}

			var (
				client kubernetes.Interface
				cfg    *rest.Config
			)
			if skipVerify || caCert != "" || clientCert != "" || clientKey != "" || secretName != "" {
				tls := &troubleshootv1beta2.TLSParams{
					SkipVerify: skipVerify,
					CACert:     caCert,
					ClientCert: clientCert,
					ClientKey:  clientKey,
				}
				if secretName != "" {
					tls.Secret = &troubleshootv1beta2.TLSSecret{Name: secretName, Namespace: secretNamespace}
					var err error
					client, cfg, err = k8sClientForCollectors()
					if err != nil {
						return err
					}
				}
				db.TLS = tls
			}

			c := &collect.CollectPostgres{Collector: db, ClientConfig: cfg, Client: client, Context: cmd.Context()}
			res, err := c.Collect(nil)
			if err != nil {
				return err
			}
			return printCollectorResult(res)
		},
	}

	f := cmd.Flags()
	f.StringVar(&uri, "uri", "", "PostgreSQL connection URI (required)")
	cmd.MarkFlagRequired("uri")
	f.BoolVar(&skipVerify, "tls-skip-verify", false, "skip TLS certificate verification")
	f.StringVar(&caCert, "tls-cacert", "", "CA certificate (PEM contents or a file path)")
	f.StringVar(&clientCert, "tls-client-cert", "", "client certificate (PEM contents or a file path)")
	f.StringVar(&clientKey, "tls-client-key", "", "client key (PEM contents or a file path)")
	f.StringVar(&secretName, "tls-secret-name", "", "name of a Secret holding TLS material (requires cluster access)")
	f.StringVar(&secretNamespace, "tls-secret-namespace", "default", "namespace of the TLS Secret")

	return cmd
}
