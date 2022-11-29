package collect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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

	cfg, err := pgx.ParseConfig(c.Collector.URI)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse postgres config")
	}

	if c.Collector.TLS != nil {
		tlsCfg, err := createTLSConfig(c.Context, c.Client, c.Collector.TLS)
		if err != nil {
			return nil, err
		}

		tlsCfg.ServerName = cfg.Host
		cfg.TLSConfig = tlsCfg
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
		databaseConnection.Error = err.Error()
	} else {
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
