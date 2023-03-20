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

	if analyzer.ClusterVersion != nil {
		isExcluded, err := isExcluded(analyzer.ClusterVersion.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzeClusterVersion(analyzer.ClusterVersion, getFile)
		if err != nil {
			return nil, err
		}
		result.Strict = analyzer.ClusterVersion.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.StorageClass != nil {
		isExcluded, err := isExcluded(analyzer.StorageClass.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzeStorageClass(analyzer.StorageClass, getFile)
		if err != nil {
			return nil, err
		}
		result.Strict = analyzer.StorageClass.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.CustomResourceDefinition != nil {
		isExcluded, err := isExcluded(analyzer.CustomResourceDefinition.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzeCustomResourceDefinition(analyzer.CustomResourceDefinition, getFile)
		if err != nil {
			return nil, err
		}
		result.Strict = analyzer.CustomResourceDefinition.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.Ingress != nil {
		isExcluded, err := isExcluded(analyzer.Ingress.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzeIngress(analyzer.Ingress, getFile)
		if err != nil {
			return nil, err
		}
		result.Strict = analyzer.Ingress.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.Secret != nil {
		isExcluded, err := isExcluded(analyzer.Secret.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzeSecret(analyzer.Secret, getFile)
		if err != nil {
			return nil, err
		}
		result.Strict = analyzer.Secret.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.ConfigMap != nil {
		isExcluded, err := isExcluded(analyzer.ConfigMap.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzeConfigMap(analyzer.ConfigMap, getFile)
		if err != nil {
			return nil, err
		}
		result.Strict = analyzer.ConfigMap.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.ImagePullSecret != nil {
		isExcluded, err := isExcluded(analyzer.ImagePullSecret.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzeImagePullSecret(analyzer.ImagePullSecret, findFiles)
		if err != nil {
			return nil, err
		}
		result.Strict = analyzer.ImagePullSecret.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.DeploymentStatus != nil {
		isExcluded, err := isExcluded(analyzer.DeploymentStatus.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		results, err := analyzeDeploymentStatus(analyzer.DeploymentStatus, findFiles)
		if err != nil {
			return nil, err
		}
		for i := range results {
			results[i].Strict = analyzer.DeploymentStatus.Strict.BoolOrDefaultFalse()
		}
		return results, nil
	}
	if analyzer.ClusterResource != nil {
		isExcluded, err := isExcluded(analyzer.ClusterResource.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzeResource(analyzer.ClusterResource, getFile)
		if err != nil {
			return nil, err
		}
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.StatefulsetStatus != nil {
		isExcluded, err := isExcluded(analyzer.StatefulsetStatus.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		results, err := analyzeStatefulsetStatus(analyzer.StatefulsetStatus, findFiles)
		if err != nil {
			return nil, err
		}
		for i := range results {
			results[i].Strict = analyzer.StatefulsetStatus.Strict.BoolOrDefaultFalse()
		}
		return results, nil
	}
	if analyzer.JobStatus != nil {
		isExcluded, err := isExcluded(analyzer.JobStatus.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		results, err := analyzeJobStatus(analyzer.JobStatus, findFiles)
		if err != nil {
			return nil, err
		}
		for i := range results {
			results[i].Strict = analyzer.JobStatus.Strict.BoolOrDefaultFalse()
		}
		return results, nil
	}
	if analyzer.ReplicaSetStatus != nil {
		isExcluded, err := isExcluded(analyzer.ReplicaSetStatus.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		results, err := analyzeReplicaSetStatus(analyzer.ReplicaSetStatus, findFiles)
		if err != nil {
			return nil, err
		}
		for i := range results {
			results[i].Strict = analyzer.ReplicaSetStatus.Strict.BoolOrDefaultFalse()
		}
		return results, nil
	}
	if analyzer.ClusterPodStatuses != nil {
		isExcluded, err := isExcluded(analyzer.ClusterPodStatuses.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		results, err := clusterPodStatuses(analyzer.ClusterPodStatuses, findFiles)
		if err != nil {
			return nil, err
		}
		for i := range results {
			results[i].Strict = analyzer.ClusterPodStatuses.Strict.BoolOrDefaultFalse()
		}
		return results, nil
	}
	if analyzer.ContainerRuntime != nil {
		isExcluded, err := isExcluded(analyzer.ContainerRuntime.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzeContainerRuntime(analyzer.ContainerRuntime, getFile)
		if err != nil {
			return nil, err
		}
		result.Strict = analyzer.ContainerRuntime.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.Distribution != nil {
		isExcluded, err := isExcluded(analyzer.Distribution.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzeDistribution(analyzer.Distribution, getFile)
		if err != nil {
			return nil, err
		}
		result.Strict = analyzer.Distribution.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.NodeResources != nil {
		isExcluded, err := isExcluded(analyzer.NodeResources.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzeNodeResources(analyzer.NodeResources, getFile)
		if err != nil {
			return nil, err
		}
		result.Strict = analyzer.NodeResources.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.TextAnalyze != nil {
		isExcluded, err := isExcluded(analyzer.TextAnalyze.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		results, err := analyzeTextAnalyze(analyzer.TextAnalyze, findFiles)
		if err != nil {
			return nil, err
		}
		for i := range results {
			results[i].Strict = analyzer.TextAnalyze.Strict.BoolOrDefaultFalse()
		}
		return results, nil
	}
	if analyzer.YamlCompare != nil {
		isExcluded, err := isExcluded(analyzer.YamlCompare.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzeYamlCompare(analyzer.YamlCompare, getFile)
		if err != nil {
			return nil, err
		}
		result.Strict = analyzer.YamlCompare.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.JsonCompare != nil {
		isExcluded, err := isExcluded(analyzer.JsonCompare.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzeJsonCompare(analyzer.JsonCompare, getFile)
		if err != nil {
			return nil, err
		}
		result.Strict = analyzer.JsonCompare.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.Postgres != nil {
		isExcluded, err := isExcluded(analyzer.Postgres.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzePostgres(analyzer.Postgres, getFile)
		if err != nil {
			return nil, err
		}
		result.Strict = analyzer.Postgres.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.Mssql != nil {
		isExcluded, err := isExcluded(analyzer.Mssql.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzeMssql(analyzer.Mssql, getFile)
		if err != nil {
			return nil, err
		}
		result.Strict = analyzer.Mssql.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.Mysql != nil {
		isExcluded, err := isExcluded(analyzer.Mysql.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzeMysql(analyzer.Mysql, getFile)
		if err != nil {
			return nil, err
		}
		result.Strict = analyzer.Mysql.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.Redis != nil {
		isExcluded, err := isExcluded(analyzer.Redis.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzeRedis(analyzer.Redis, getFile)
		if err != nil {
			return nil, err
		}
		result.Strict = analyzer.Redis.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.CephStatus != nil {
		isExcluded, err := isExcluded(analyzer.CephStatus.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := cephStatus(analyzer.CephStatus, getFile)
		if err != nil {
			return nil, err
		}
		if result != nil {
			result.Strict = analyzer.CephStatus.Strict.BoolOrDefaultFalse()
		}
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.Longhorn != nil {
		isExcluded, err := isExcluded(analyzer.Longhorn.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		results, err := longhorn(analyzer.Longhorn, getFile, findFiles)
		if err != nil {
			return nil, err
		}
		for i := range results {
			results[i].Strict = analyzer.Longhorn.Strict.BoolOrDefaultFalse()
		}
		return results, nil
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
