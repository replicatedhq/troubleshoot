package collect

import (
	"database/sql"
	"encoding/json"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

func Mysql(c *Collector, databaseCollector *troubleshootv1beta2.Database) (map[string][]byte, error) {
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
	}

	b, err := json.Marshal(databaseConnection)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal database connection")
	}

	collectorName := databaseCollector.CollectorName
	if collectorName == "" {
		collectorName = "mysql"
	}

	mysqlOutput := map[string][]byte{
		fmt.Sprintf("mysql/%s.json", collectorName): b,
	}

	return mysqlOutput, nil
}
