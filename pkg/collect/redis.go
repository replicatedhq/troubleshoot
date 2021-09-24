package collect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-redis/redis/v7"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

func Redis(c *Collector, databaseCollector *troubleshootv1beta2.Database) (CollectorResult, error) {
	databaseConnection := DatabaseConnection{}

	opt, err := redis.ParseURL(databaseCollector.URI)
	if err != nil {
		databaseConnection.Error = err.Error()
	} else {
		client := redis.NewClient(opt)
		stringResult := client.Info("server")

		if stringResult.Err() != nil {
			databaseConnection.Error = stringResult.Err().Error()
		}

		databaseConnection.IsConnected = stringResult.Err() == nil

		if databaseConnection.Error == "" {
			lines := strings.Split(stringResult.Val(), "\n")
			for _, line := range lines {
				lineParts := strings.Split(line, ":")
				if len(lineParts) == 2 {
					if lineParts[0] == "redis_version" {
						databaseConnection.Version = strings.TrimSpace(lineParts[1])
					}
				}
			}
		}
	}

	b, err := json.Marshal(databaseConnection)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal database connection")
	}

	collectorName := databaseCollector.CollectorName
	if collectorName == "" {
		collectorName = "redis"
	}

	output := NewResult()
	output.SaveResult(c.BundlePath, fmt.Sprintf("redis/%s.json", collectorName), bytes.NewBuffer(b))

	return output, nil
}
