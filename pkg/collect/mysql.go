package collect

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

func Mysql(c *Collector, databaseCollector *troubleshootv1beta2.Database) (CollectorResult, error) {
	databaseConnection := DatabaseConnection{}

	db, err := sql.Open("mysql", databaseCollector.URI)
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
			databaseConnection.Variables = variables
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
