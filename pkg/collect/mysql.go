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
		}
		columns, err := rows.Columns()
		if err != nil {
			databaseConnection.Error = err.Error()
		}
		values := make([]sql.RawBytes, len(columns))
		scanArgs := make([]interface{}, len(values))
		for i := range values {
			scanArgs[i] = &values[i]
		}

		variables := make(map[string]string)
		for rows.Next() {
			err = rows.Scan(scanArgs...)
			if err != nil {
				databaseConnection.Error = err.Error()
			}

			key := string(values[0])
			var value string
			if string(values[1]) == "" {
				value = "NULL"
			} else {
				value = string(values[1])
			}
			variables[key] = value
		}
		databaseConnection.Variables = variables
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
