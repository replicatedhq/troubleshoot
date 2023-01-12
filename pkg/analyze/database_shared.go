package analyzer

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

func compareDatabaseConditionalToActual(conditional string, result *collect.DatabaseConnection) (bool, error) {
	parts := strings.Split(strings.TrimSpace(conditional), " ")

	if len(parts) != 3 {
		return false, errors.New("unable to parse conditional")
	}

	switch parts[0] {
	case "connected":
		expected, err := strconv.ParseBool(parts[2])
		if err != nil {
			return false, errors.Wrap(err, "failed to parse bool")
		}

		switch parts[1] {
		case "=", "==", "===":
			return expected == result.IsConnected, nil
		case "!=", "!==":
			return expected != result.IsConnected, nil

		}

		return false, errors.New("unable to parse postgres connected analyzer")

	case "version":
		expected, err := semver.ParseTolerant(strings.Replace(parts[2], "x", "0", -1))
		if err != nil {
			return false, errors.Wrap(err, "failed to parse expected version")
		}
		actual, err := semver.ParseTolerant(strings.Replace(result.Version, "x", "0", -1))
		if err != nil {
			return false, errors.Wrap(err, "failed to parse postgres db actual version")
		}

		expectedRange, err := semver.ParseRange(fmt.Sprintf("%s %s", parts[1], expected.String()))
		if err != nil {
			return false, errors.Wrap(err, "failed to parse semver range")
		}

		return expectedRange(actual), nil
	}

	return false, nil
}
