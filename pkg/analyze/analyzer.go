package analyzer

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	"github.com/replicatedhq/troubleshoot/pkg/multitype"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	corev1 "k8s.io/api/core/v1"
)

type AnalyzeResult struct {
	IsPass bool
	IsFail bool
	IsWarn bool
	Strict bool

	Title   string
	Message string
	URI     string
	IconKey string
	IconURI string

	InvolvedObject *corev1.ObjectReference
}

type getCollectedFileContents func(string) ([]byte, error)
type getChildCollectedFileContents func(string, []string) (map[string][]byte, error)

func isExcluded(excludeVal *multitype.BoolOrString) (bool, error) {
	if excludeVal == nil {
		return false, nil
	}

	if excludeVal.Type == multitype.Bool {
		return excludeVal.BoolVal, nil
	}

	if excludeVal.StrVal == "" {
		return false, nil
	}

	parsed, err := strconv.ParseBool(excludeVal.StrVal)
	if err != nil {
		return false, errors.Wrap(err, "failed to parse bool string")
	}

	return parsed, nil
}

func HostAnalyze(
	ctx context.Context,
	hostAnalyzer *troubleshootv1beta2.HostAnalyze,
	getFile getCollectedFileContents,
	findFiles getChildCollectedFileContents,
) []*AnalyzeResult {
	analyzer, ok := GetHostAnalyzer(hostAnalyzer)
	if !ok {
		return NewAnalyzeResultError(analyzer, errors.New("invalid host analyzer"))
	}

	_, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, analyzer.Title())
	span.SetAttributes(attribute.String("type", reflect.TypeOf(analyzer).String()))
	defer span.End()

	isExcluded, _ := analyzer.IsExcluded()
	if isExcluded {
		logger.Printf("Excluding %q analyzer", analyzer.Title())
		span.SetAttributes(attribute.Bool(constants.EXCLUDED, true))
		return nil
	}

	result, err := analyzer.Analyze(getFile)
	if err != nil {
		return NewAnalyzeResultError(analyzer, errors.Wrap(err, "analyze"))
	}
	return result
}

func NewAnalyzeResultError(analyzer HostAnalyzer, err error) []*AnalyzeResult {
	if analyzer != nil {
		return []*AnalyzeResult{{
			IsFail:  true,
			Title:   analyzer.Title(),
			Message: fmt.Sprintf("Analyzer Failed: %v", err),
		}}
	}
	return []*AnalyzeResult{{
		IsFail:  true,
		Title:   "nil analyzer",
		Message: fmt.Sprintf("Analyzer Failed: %v", err),
	}}
}

func Analyze(
	ctx context.Context,
	analyzer *troubleshootv1beta2.Analyze,
	getFile getCollectedFileContents,
	findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {
	if analyzer == nil {
		return nil, errors.New("nil analyzer")
	}

	analyzerInst := getAnalyzer(analyzer)
	if analyzerInst == nil {
		return []*AnalyzeResult{{
			IsFail:  true,
			Title:   "nonexistent analyzer",
			Message: "Analyzer not found",
		}}, nil
	}

	_, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, analyzerInst.Title())
	span.SetAttributes(attribute.String("type", reflect.TypeOf(analyzerInst).String()))
	defer span.End()

	isExcluded, err := analyzerInst.IsExcluded()
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	if isExcluded {
		logger.Printf("Excluding %q analyzer", analyzerInst.Title())
		span.SetAttributes(attribute.Bool(constants.EXCLUDED, true))
		return nil, nil
	}

	results, err := analyzerInst.Analyze(getFile, findFiles)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	if results == nil {
		results = []*AnalyzeResult{}
	}

	return results, nil
}

func GetExcludeFlag(analyzer *troubleshootv1beta2.Analyze) *multitype.BoolOrString {
	if analyzer == nil {
		return nil
	}

	reflected := reflect.ValueOf(analyzer).Elem()
	for i := 0; i < reflected.NumField(); i++ {
		if reflected.Field(i).IsNil() {
			continue
		}

		field := reflect.Indirect(reflected.Field(i)).FieldByName("Exclude")
		exclude, ok := field.Interface().(*multitype.BoolOrString)
		if !ok {
			continue
		}
		return exclude
	}

	return nil
}

type Analyzer interface {
	Title() string
	IsExcluded() (bool, error)
	Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error)
}

func getAnalyzer(analyzer *troubleshootv1beta2.Analyze) Analyzer {
	switch {
	case analyzer.ClusterVersion != nil:
		return &AnalyzeClusterVersion{analyzer: analyzer.ClusterVersion}
	case analyzer.StorageClass != nil:
		return &AnalyzeStorageClass{analyzer: analyzer.StorageClass}
	case analyzer.CustomResourceDefinition != nil:
		return &AnalyzeCustomResourceDefinition{analyzer: analyzer.CustomResourceDefinition}
	case analyzer.Ingress != nil:
		return &AnalyzeIngress{analyzer: analyzer.Ingress}
	case analyzer.Secret != nil:
		return &AnalyzeSecret{analyzer: analyzer.Secret}
	case analyzer.ConfigMap != nil:
		return &AnalyzeConfigMap{analyzer: analyzer.ConfigMap}
	case analyzer.ImagePullSecret != nil:
		return &AnalyzeImagePullSecret{analyzer: analyzer.ImagePullSecret}
	case analyzer.DeploymentStatus != nil:
		return &AnalyzeDeploymentStatus{analyzer: analyzer.DeploymentStatus}
	case analyzer.StatefulsetStatus != nil:
		return &AnalyzeStatefulsetStatus{analyzer: analyzer.StatefulsetStatus}
	case analyzer.JobStatus != nil:
		return &AnalyzeJobStatus{analyzer: analyzer.JobStatus}
	case analyzer.ReplicaSetStatus != nil:
		return &AnalyzeReplicaSetStatus{analyzer: analyzer.ReplicaSetStatus}
	case analyzer.ClusterPodStatuses != nil:
		return &AnalyzeClusterPodStatuses{analyzer: analyzer.ClusterPodStatuses}
	case analyzer.ContainerRuntime != nil:
		return &AnalyzeContainerRuntime{analyzer: analyzer.ContainerRuntime}
	case analyzer.Distribution != nil:
		return &AnalyzeDistribution{analyzer: analyzer.Distribution}
	case analyzer.NodeResources != nil:
		return &AnalyzeNodeResources{analyzer: analyzer.NodeResources}
	case analyzer.TextAnalyze != nil:
		return &AnalyzeTextAnalyze{analyzer: analyzer.TextAnalyze}
	case analyzer.YamlCompare != nil:
		return &AnalyzeYamlCompare{analyzer: analyzer.YamlCompare}
	case analyzer.JsonCompare != nil:
		return &AnalyzeJsonCompare{analyzer: analyzer.JsonCompare}
	case analyzer.Postgres != nil:
		return &AnalyzePostgres{analyzer: analyzer.Postgres}
	case analyzer.Mysql != nil:
		return &AnalyzeMysql{analyzer: analyzer.Mysql}
	case analyzer.Redis != nil:
		return &AnalyzeRedis{analyzer: analyzer.Redis}
	case analyzer.CephStatus != nil:
		return &AnalyzeCephStatus{analyzer: analyzer.CephStatus}
	case analyzer.Longhorn != nil:
		return &AnalyzeLonghorn{analyzer: analyzer.Longhorn}
	case analyzer.RegistryImages != nil:
		return &AnalyzeRegistryImages{analyzer: analyzer.RegistryImages}
	case analyzer.WeaveReport != nil:
		return &AnalyzeWeaveReport{analyzer: analyzer.WeaveReport}
	case analyzer.Sysctl != nil:
		return &AnalyzeSysctl{analyzer: analyzer.Sysctl}
	case analyzer.ClusterResource != nil:
		return &AnalyzeClusterResource{analyzer: analyzer.ClusterResource}
	default:
		return nil
	}
}
