package collect

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"

	_ "github.com/microsoft/go-mssqldb"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
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
	return getCollectorName(c)
}

func (c *CollectMssql) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectMssql) SkipRedaction() bool {
	return c.Collector.SkipRedaction
}

func (c *CollectMssql) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	databaseConnection := DatabaseConnection{}

	connUrl, err := url.Parse(c.Collector.URI)
	// Parsing the uri should not lead to the collector failing
	// sql.Open will fail if the uri is invalid.
	if err == nil {
		klog.V(2).Infof("Connect to %q MSSQL Database", connUrl.Host)
	} else {
		klog.V(2).Info("Connect to MSSQL Database")
	}

	db, err := sql.Open("mssql", c.Collector.URI)
	if err != nil {
		klog.V(2).Infof("Failed to connect to %q MSSQL Database: %v", connUrl.Host, err)
		databaseConnection.Error = err.Error()
	} else {
		defer db.Close()
		query := `select @@VERSION as version`
		row := db.QueryRow(query)
		version := ""
		if err := row.Scan(&version); err != nil {
			klog.V(2).Infof("Failed to query version string from database: %s", err)
			databaseConnection.Error = err.Error()
		} else {
			databaseConnection.IsConnected = true

			mssqlVersion, err := parseMsSqlVersion(version)
			if err != nil {
				databaseConnection.Version = "Unknown"
				databaseConnection.Error = err.Error()
			} else {
				databaseConnection.Version = mssqlVersion
			}
			klog.V(2).Infof("Successfully queried version string from database: %s", version)
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

func parseMsSqlVersion(mssqlVersion string) (string, error) {
	re, err := regexp.Compile(`.*SQL.*-\s+([0-9.]+)`)
	if err != nil {
		return "", errors.Wrap(err, "failed to compile regex")
	}
	matches := re.FindStringSubmatch(mssqlVersion)
	if len(matches) < 2 {
		return "", errors.Errorf("mssql version did not match regex: %q", mssqlVersion)
	}

	return matches[1], nil
}
