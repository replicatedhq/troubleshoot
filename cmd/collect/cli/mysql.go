package cli

import (
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// MysqlCmd runs the mysql collector against a single database and prints the
// native result JSON ({"isConnected":..,"version":..,"variables":..,"error":..}).
func MysqlCmd() *cobra.Command {
	var (
		uri             string
		parameters      []string
		skipVerify      bool
		caCert          string
		clientCert      string
		clientKey       string
		secretName      string
		secretNamespace string
	)

	cmd := &cobra.Command{
		Use:   "mysql",
		Short: "Run the mysql collector against a database",
		Long: `Run the mysql collector: connect to a MySQL server, run "select version()",
optionally collect server variables, and print the collector result as JSON.

--uri is a go-sql-driver DSN (note: not a URL):
  user:password@tcp(host:3306)/dbname

--parameters names server variables to collect via "SHOW VARIABLES"; the matching
values are returned under "variables" in the result. This flag is unique to mysql.

Example:
  collect mysql --uri "root:pass@tcp(mysql.default.svc:3306)/" --parameters max_connections,version`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			db := &troubleshootv1beta2.Database{
				CollectorMeta: troubleshootv1beta2.CollectorMeta{CollectorName: "mysql"},
				URI:           uri,
				Parameters:    parameters,
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

			c := &collect.CollectMysql{Collector: db, ClientConfig: cfg, Client: client, Context: cmd.Context()}
			res, err := c.Collect(nil)
			if err != nil {
				return err
			}
			return printCollectorResult(res)
		},
	}

	f := cmd.Flags()
	f.StringVar(&uri, "uri", "", "MySQL connection DSN, e.g. user:pass@tcp(host:3306)/db (required)")
	cmd.MarkFlagRequired("uri")
	f.StringSliceVar(&parameters, "parameters", nil, "server variables to collect via SHOW VARIABLES (comma-separated or repeatable)")
	f.BoolVar(&skipVerify, "tls-skip-verify", false, "skip TLS certificate verification")
	f.StringVar(&caCert, "tls-cacert", "", "CA certificate (PEM contents or a file path)")
	f.StringVar(&clientCert, "tls-client-cert", "", "client certificate (PEM contents or a file path)")
	f.StringVar(&clientKey, "tls-client-key", "", "client key (PEM contents or a file path)")
	f.StringVar(&secretName, "tls-secret-name", "", "name of a Secret holding TLS material (requires cluster access)")
	f.StringVar(&secretNamespace, "tls-secret-namespace", "default", "namespace of the TLS Secret")

	return cmd
}
