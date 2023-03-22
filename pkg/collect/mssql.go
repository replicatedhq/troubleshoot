package collect

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"

	_ "github.com/microsoft/go-mssqldb"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type CollectMssql struct {
	Collector    *troubleshootv1beta2.Database
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

func (c *CollectMssql) Title() string {
	return collectorTitleOrDefault(c.Collector.CollectorMeta, "MSSSQLServer")
}

func (c *CollectMssql) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectMssql) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	databaseConnection := DatabaseConnection{}

	db, err := sql.Open("mssql", c.Collector.URI)
	if err != nil {
		databaseConnection.Error = err.Error()
	} else {
		query := `select @@VERSION as version`
		row := db.QueryRow(query)
		version := ""
		if err := row.Scan(&version); err != nil {
			databaseConnection.Error = err.Error()
		} else {
			databaseConnection.IsConnected = true

<<<<<<< HEAD
			mssqlVersion, err := parseMsSqlVersion(version)
=======
			mssqlVersion, err := parseMSSqlVersion(version)
>>>>>>> ffcf962 (Adds MSSQL collector based on Postgres collector)
			if err != nil {
				databaseConnection.Version = "Unknown"
				databaseConnection.Error = err.Error()
			} else {
				databaseConnection.Version = mssqlVersion
			}
		}
	}

	b, err := json.Marshal(databaseConnection)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal database connection")
	}

	collectorName := c.Collector.CollectorName
	if collectorName == "" {
		collectorName = "mssql"
	}

	output := NewResult()
	output.SaveResult(c.BundlePath, fmt.Sprintf("mssql/%s.json", collectorName), bytes.NewBuffer(b))

	return output, nil
}

<<<<<<< HEAD
func parseMsSqlVersion(mssqlVersion string) (string, error) {
	re := regexp.MustCompile(".*SQL.*-\\s+([0-9.]+)")
=======
func parseMSSqlVersion(mssqlVersion string) (string, error) {
	re := regexp.MustCompile("MSSQLServer ([0-9.]*)")
>>>>>>> ffcf962 (Adds MSSQL collector based on Postgres collector)
	matches := re.FindStringSubmatch(mssqlVersion)
	if len(matches) < 2 {
		return "", errors.Errorf("mssql version did not match regex: %q", mssqlVersion)
	}

	return matches[1], nil

}
