package collect

import (
	"bytes"
  "context"
	"database/sql"
	"encoding/json"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func Mysql(c *Collector, databaseCollector *troubleshootv1beta2.Database) (CollectorResult, error) {
	databaseConnection := DatabaseConnection{}

	uri, err := getUri(c.ClientConfig, databaseCollector)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("mysql", uri)
	if err != nil {
		databaseConnection.Error = err.Error()
	} else {
		defer db.Close()
		query := `select version()`
		row := db.QueryRow(query)

		version := ""
		if err := row.Scan(&version); err != nil {
			databaseConnection.Error = err.Error()
		} else {
			databaseConnection.IsConnected = true
			databaseConnection.Version = version
		}

		requestedParameters := databaseCollector.Parameters
		if len(requestedParameters) > 0 {
			rows, err := db.Query("SHOW VARIABLES")

			if err != nil {
				databaseConnection.Error = err.Error()
			} else {
				defer rows.Close()

				variables := map[string]string{}
				for rows.Next() {
					var key, value string
					err = rows.Scan(&key, &value)
					if err != nil {
						databaseConnection.Error = err.Error()
						break
					}
					variables[key] = value
				}
				filteredVariables := map[string]string{}

				for _, key := range requestedParameters {
					if value, ok := variables[key]; ok {
						filteredVariables[key] = value
					}

				}
				databaseConnection.Variables = filteredVariables
			}
		}

	}

	b, err := json.Marshal(databaseConnection)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal database connection")
	}

	collectorName := databaseCollector.CollectorName
	if collectorName == "" {
		collectorName = "mysql"
	}

	output := NewResult()
	output.SaveResult(c.BundlePath, fmt.Sprintf("mysql/%s.json", collectorName), bytes.NewBuffer(b))

	return output, nil
}

func getUri(clientConfig *rest.Config, databaseCollector *troubleshootv1beta2.Database) (string, error) {
	if databaseCollector.URI.Value != "" {
		return databaseCollector.URI.Value, nil
	} else if databaseCollector.URI.ValueFrom != nil {
		if databaseCollector.URI.ValueFrom.SecretKeyRef != nil {
			if databaseCollector.URI.ValueFrom.SecretKeyRef.Namespace == "" {
				databaseCollector.URI.ValueFrom.SecretKeyRef.Namespace = "default"
			}
			client, err := kubernetes.NewForConfig(clientConfig)
			if err != nil {
				return "", err
			}
			ctx := context.Background()
			found, err := client.CoreV1().Secrets(databaseCollector.URI.ValueFrom.SecretKeyRef.Namespace).Get(ctx, databaseCollector.URI.ValueFrom.SecretKeyRef.Name, metav1.GetOptions{})
			if err != nil {
				return "", err
			}
			if val, ok := found.Data[databaseCollector.URI.ValueFrom.SecretKeyRef.Key]; ok {
				return string(val), nil
			}
			return "", errors.Errorf("Secret Key %s not found", databaseCollector.URI.ValueFrom.SecretKeyRef.Key)

		}
	}
	return "", errors.Errorf("A connection uri must be provided")

}
