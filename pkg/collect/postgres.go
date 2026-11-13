package collect

import (
	"bytes"
	"context"
	"crypto/tls"
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

	var clientCertPair *tls.Certificate

	if c.Collector.TLS != nil {
		klog.V(2).Infof("Connecting to postgres with TLS client config")
		caCert, clientCert, clientKey, err := getTLSParamTriplet(c.Context, c.Client, c.Collector.TLS)
		if err != nil {
			return nil, err
		}

		if caCert != "" {
			// Set the libpq root certificate environment variable since pgx
			// parses it to build the root CA pool of the TLS configuration
			// (tls.Config instance) used to connect with. pgx reads the CA cert
			// eagerly while parsing the connection config, and its presence
			// preserves libpq sslmode semantics (e.g. sslmode=require behaves
			// like verify-ca when a root certificate is configured).
			// https://www.postgresql.org/docs/current/libpq-envars.html
			// The environment variable is unset and the file deleted after the
			// connection config is created.
			tmpdir, err := os.MkdirTemp("", "ts-postgres-collector")
			if err != nil {
				return nil, errors.Wrap(err, "failed to create temp dir to store postgres collector TLS files")
			}
			defer os.RemoveAll(tmpdir)

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

		if clientCert != "" || clientKey != "" {
			// pgx v5.13.0 no longer stores file-backed client certificates
			// (PGSSLCERT/PGSSLKEY) in the parsed TLSConfig; it re-reads the files
			// on every TLS handshake. Temp files written here are deleted before
			// the connection is established, so load the pair in memory instead
			// and set it on the parsed TLS configs after parsing. Setting
			// TLSConfig.Certificates is the override pgx documents for its
			// automatic client certificate reloading.
			if clientCert == "" || clientKey == "" {
				return nil, errors.New("both client cert and client key are required to configure postgres TLS")
			}
			pair, err := tls.X509KeyPair([]byte(clientCert), []byte(clientKey))
			if err != nil {
				return nil, errors.Wrap(err, "failed to load client cert and key")
			}
			clientCertPair = &pair
		}
	}

	cfg, err := pgx.ParseConfig(c.Collector.URI)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse postgres config")
	}
	klog.V(2).Infof("Successfully parsed postgres config")

	if clientCertPair != nil {
		// The primary TLS config and any per-host fallbacks each get their
		// own tls.Config instance; set the pair on all of them.
		tlsConfigs := []*tls.Config{cfg.TLSConfig}
		for _, fallback := range cfg.Fallbacks {
			if fallback.TLSConfig != nil {
				tlsConfigs = append(tlsConfigs, fallback.TLSConfig)
			}
		}
		for _, tlsConfig := range tlsConfigs {
			if tlsConfig != nil {
				tlsConfig.Certificates = []tls.Certificate{*clientCertPair}
			}
		}
	}

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
