package collect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

type CollectPostgres struct {
	Collector    *troubleshootv1beta2.Database
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

func (c *CollectPostgres) Title() string {
	return getCollectorName(c)
}

func (c *CollectPostgres) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectPostgres) createConnectConfig() (*pgx.ConnConfig, error) {
	if c.Collector.URI == "" {
		return nil, errors.New("postgres uri cannot be empty")
	}

	if c.Collector.TLS != nil {
		klog.V(2).Infof("Connecting to postgres with TLSCertificate client config")
		// Set the libpq TLSCertificate environment variables since pgx parses them to
		// create the TLSCertificate configuration (tls.Config instance) to connect with
		// https://www.postgresql.org/docs/current/libpq-envars.html
		caCert, clientCert, clientKey, err := getTLSParamTriplet(c.Context, c.Client, c.Collector.TLS)
		if err != nil {
			return nil, err
		}

		// Drop the TLSCertificate params to files and set the paths to their
		// respective environment variables
		// The environment variables are unset after the connection config
		// is created. Their respective files are deleted as well.
		tmpdir, err := os.MkdirTemp("", "ts-postgres-collector")
		if err != nil {
			return nil, errors.Wrap(err, "failed to create temp dir to store postgres collector TLSCertificate files")
		}
		defer os.RemoveAll(tmpdir)

		if caCert != "" {
			caCertPath := filepath.Join(tmpdir, "ca.crt")
			err = os.WriteFile(caCertPath, []byte(caCert), 0644)
			if err != nil {
				return nil, errors.Wrap(err, "failed to write ca cert to file")
			}
			err = os.Setenv("PGSSLROOTCERT", caCertPath)
			if err != nil {
				return nil, errors.Wrap(err, "failed to set PGSSLROOTCERT environment variable")
			}
			klog.V(2).Infof("'PGSSLROOTCERT' environment variable set to %q", caCertPath)
			defer os.Unsetenv("PGSSLROOTCERT")
		}

		if clientCert != "" {
			clientCertPath := filepath.Join(tmpdir, "client.crt")
			err = os.WriteFile(clientCertPath, []byte(clientCert), 0644)
			if err != nil {
				return nil, errors.Wrap(err, "failed to write client cert to file")
			}
			err = os.Setenv("PGSSLCERT", clientCertPath)
			if err != nil {
				return nil, errors.Wrap(err, "failed to set PGSSLCERT environment variable")
			}
			klog.V(2).Infof("'PGSSLCERT' environment variable set to %q", clientCertPath)
			defer os.Unsetenv("PGSSLCERT")
		}

		if clientKey != "" {
			clientKeyPath := filepath.Join(tmpdir, "client.key")
			err = os.WriteFile(clientKeyPath, []byte(clientKey), 0600)
			if err != nil {
				return nil, errors.Wrap(err, "failed to write client key to file")
			}
			err = os.Setenv("PGSSLKEY", clientKeyPath)
			if err != nil {
				return nil, errors.Wrap(err, "failed to set PGSSLKEY environment variable")
			}
			klog.V(2).Infof("'PGSSLKEY' environment variable set to %q", clientKeyPath)
			defer os.Unsetenv("PGSSLKEY")
		}
	}

	cfg, err := pgx.ParseConfig(c.Collector.URI)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse postgres config")
	}
	klog.V(2).Infof("Successfully parsed postgres config")

	return cfg, nil
}

func (c *CollectPostgres) connect() (*pgx.Conn, error) {
	connCfg, err := c.createConnectConfig()
	if err != nil {
		return nil, err
	}

	conn, err := pgx.ConnectConfig(c.Context, connCfg)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (c *CollectPostgres) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	databaseConnection := DatabaseConnection{}

	conn, err := c.connect()
	if err != nil {
		klog.V(2).Infof("Postgres connection error: %s", err.Error())
		databaseConnection.Error = err.Error()
	} else {
		klog.V(2).Info("Successfully connected to postgres")
		defer conn.Close(c.Context)

		query := `select version()`
		row := conn.QueryRow(c.Context, query)
		version := ""
		if err := row.Scan(&version); err != nil {
			databaseConnection.Error = err.Error()
		} else {
			databaseConnection.IsConnected = true

			postgresVersion, err := parsePostgresVersion(version)
			if err != nil {
				databaseConnection.Version = "Unknown"
				databaseConnection.Error = err.Error()
			} else {
				databaseConnection.Version = postgresVersion
			}
		}
	}

	b, err := json.Marshal(databaseConnection)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal database connection")
	}

	collectorName := c.Collector.CollectorName
	if collectorName == "" {
		collectorName = "postgres"
	}

	output := NewResult()
	output.SaveResult(c.BundlePath, fmt.Sprintf("postgres/%s.json", collectorName), bytes.NewBuffer(b))

	return output, nil
}

func parsePostgresVersion(postgresVersion string) (string, error) {
	re := regexp.MustCompile("PostgreSQL ([0-9.]*)")
	matches := re.FindStringSubmatch(postgresVersion)
	if len(matches) < 2 {
		return "", errors.Errorf("postgres version did not match regex: %q", postgresVersion)
	}

	return matches[1], nil

}
