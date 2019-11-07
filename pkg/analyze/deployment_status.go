package analyzer

import (
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
)

func deploymentStatus(analyzer *troubleshootv1beta1.DeploymentStatus, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	collected, err := getCollectedFileContents(path.Join("cluster-resources", "deployments", fmt.Sprintf("%s.json", analyzer.Namespace)))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read collected deployments from namespace")
	}

	var deployments []appsv1.Deployment
	if err := json.Unmarshal(collected, &deployments); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal deployment list")
	}

	var status *appsv1.DeploymentStatus
	if analyzer.Name != "" {
		for _, deployment := range deployments {
			if deployment.Name == analyzer.Name {
				status = &deployment.Status
			}
		}
	} else if analyzer.Selector != nil {

	}

	if status == nil {
		// there's not an error, but maybe the requested deployment is not even deployed
		return &AnalyzeResult{
			IsFail:  true,
			Message: "not found",
		}, nil
	}

	result := &AnalyzeResult{}

	// ordering from the spec is important, the first one that matches returns
	for _, outcome := range analyzer.Outcomes {
		if outcome.Fail != nil {
			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return result, nil
			}

			match, err := compareActualToWhen(outcome.Fail.When, int(status.ReadyReplicas))
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse fail range")
			}

			if match {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return result, nil
			}
		} else if outcome.Warn != nil {
			if outcome.Warn.When == "" {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return result, nil
			}

			match, err := compareActualToWhen(outcome.Warn.When, int(status.ReadyReplicas))
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse warn range")
			}

			if match {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return result, nil
			}
		} else if outcome.Pass != nil {
			if outcome.Pass.When == "" {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return result, nil
			}

			match, err := compareActualToWhen(outcome.Pass.When, int(status.ReadyReplicas))
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse pass range")
			}

			if match {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return result, nil
			}
		}
	}

	return result, nil
}

func compareActualToWhen(when string, actual int) (bool, error) {
	parts := strings.Split(strings.TrimSpace(when), " ")

	// we can make this a lot more flexible
	if len(parts) != 2 {
		return false, errors.New("unable to parse when range")
	}

	value, err := strconv.Atoi(parts[1])
	if err != nil {
		return false, errors.New("unable to parse when value")
	}

	switch parts[0] {
	case "=":
		fallthrough
	case "==":
		fallthrough
	case "===":
		return actual == value, nil

	case "<":
		return actual < value, nil

	case ">":
		return actual > value, nil

	case "<=":
		return actual <= value, nil

	case ">=":
		return actual >= value, nil
	}

	return false, errors.Errorf("unknown comparator: %q", parts[0])
}
