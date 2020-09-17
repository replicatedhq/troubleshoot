package analyzer

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/blang/semver"
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
		//semver requires major.minor.patch format to successqfully compare versions.
		if compVer := strings.Split(parts[2], "."); len(compVer) == 2 {
			parts[2] = fmt.Sprintf("%s.%s.0", compVer[0], compVer[1])
		} else if len(compVer) == 1 {
			parts[2] = fmt.Sprintf("%s.0.0", compVer[0])
		}
		if compVer := strings.Split(result.Version, "."); len(compVer) == 2 {
			result.Version = fmt.Sprintf("%s.%s.0", compVer[0], compVer[1])
		} else if len(compVer) == 1 {
			result.Version = fmt.Sprintf("%s.0.0", compVer[0])
		}

		expectedRange, err := semver.ParseRange(fmt.Sprintf("%s %s", parts[1], parts[2]))
		if err != nil {
			return false, errors.Wrap(err, "failed to parse semver range")
		}

		actual, err := semver.Parse(result.Version)
		if err != nil {
			return false, errors.Wrap(err, "failed to parse actual postgres version")
		}

		return expectedRange(actual), nil
	}

	return false, nil
}
