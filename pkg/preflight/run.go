package preflight

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	cursor "github.com/ahmetalpbalkan/go-cursor"
	"github.com/fatih/color"
	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/internal/util"
	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/replicatedhq/troubleshoot/pkg/types"
	"github.com/spf13/viper"
	spin "github.com/tj/go-spin"
	"go.opentelemetry.io/otel"
	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func RunPreflights(interactive bool, output string, format string, args []string) error {
	ctx, root := otel.Tracer(
		constants.LIB_TRACER_NAME).Start(context.Background(), constants.TROUBLESHOOT_ROOT_SPAN_NAME)
	defer root.End()

	if interactive {
		fmt.Print(cursor.Hide())
		defer fmt.Print(cursor.Show())
	}

	go func() {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt)
		<-signalChan
		// exiting due to a signal shouldn't be considered successful
		os.Exit(1)
	}()

	specs, err := readSpecs(args)
	if err != nil {
		return types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, err)
	}

	if interactive {
		if len(specs.HostPreflightsV1Beta2) > 0 && !util.IsRunningAsRoot() {
			if util.PromptYesNo("Some host collectors may require elevated privileges to run.\nDo you want to exit and rerun the command as a privileged user?") {
				fmt.Println("Exiting...")
				return nil
			}
		}
	}

	warning := validatePreflight(specs)
	if warning != nil {
		fmt.Println(warning.Warning())
		return nil
	}

	if viper.GetBool("dry-run") {
		out, err := specs.ToYaml()
		if err != nil {
			return types.NewExitCodeError(constants.EXIT_CODE_CATCH_ALL, errors.Wrap(err, "failed to convert specs to yaml"))
		}
		fmt.Printf("%s", out)
		return nil
	}

	var collectResults []CollectResult
	var uploadCollectResults []CollectResult
	preflightSpecName := ""

	progressCh := make(chan interface{})
	defer close(progressCh)

	ctx, stopProgressCollection := context.WithCancel(ctx)
	// make sure we shut down progress collection goroutines if an error occurs
	defer stopProgressCollection()
	progressCollection, ctx := errgroup.WithContext(ctx)

	if interactive {
		progressCollection.Go(collectInteractiveProgress(ctx, progressCh))
	} else {
		progressCollection.Go(collectNonInteractiveProgess(ctx, progressCh))
	}

	uploadResultsMap := make(map[string][]CollectResult)

	for _, spec := range specs.PreflightsV1Beta2 {
		r, err := collectInCluster(ctx, &spec, progressCh)
		if err != nil {
			return types.NewExitCodeError(constants.EXIT_CODE_CATCH_ALL, errors.Wrap(err, "failed to collect in cluster"))
		}
		if spec.Spec.UploadResultsTo != "" {
			uploadResultsMap[spec.Spec.UploadResultsTo] = append(uploadResultsMap[spec.Spec.UploadResultsTo], *r)
			uploadCollectResults = append(collectResults, *r)
		} else {
			collectResults = append(collectResults, *r)
		}
		// TODO: This spec name will be overwritten by the next spec. Is this intentional?
		preflightSpecName = spec.Name
	}

	for _, spec := range specs.HostPreflightsV1Beta2 {
		if len(spec.Spec.Collectors) > 0 {
			r, err := collectHost(ctx, &spec, progressCh)
			if err != nil {
				return types.NewExitCodeError(constants.EXIT_CODE_CATCH_ALL, errors.Wrap(err, "failed to collect from host"))
			}
			collectResults = append(collectResults, *r)
		}
		if len(spec.Spec.RemoteCollectors) > 0 {
			r, err := collectRemote(ctx, &spec, progressCh)
			if err != nil {
				return types.NewExitCodeError(constants.EXIT_CODE_CATCH_ALL, errors.Wrap(err, "failed to collect remotely"))
			}
			collectResults = append(collectResults, *r)
		}
		preflightSpecName = spec.Name
	}

	if len(collectResults) == 0 && len(uploadCollectResults) == 0 {
		return types.NewExitCodeError(constants.EXIT_CODE_CATCH_ALL, errors.New("no data was collected"))
	}

	analyzeResults := []*analyzer.AnalyzeResult{}
	for _, res := range collectResults {
		analyzeResults = append(analyzeResults, res.Analyze()...)
	}

	uploadAnalyzeResultsMap := make(map[string][]*analyzer.AnalyzeResult)
	for location, results := range uploadResultsMap {
		for _, res := range results {
			uploadAnalyzeResultsMap[location] = append(uploadAnalyzeResultsMap[location], res.Analyze()...)
			analyzeResults = append(analyzeResults, uploadAnalyzeResultsMap[location]...)
		}
	}

	for k, v := range uploadAnalyzeResultsMap {
		err := uploadResults(k, v)
		if err != nil {
			progressCh <- err
		}
	}

	stopProgressCollection()
	progressCollection.Wait()

	if len(analyzeResults) == 0 {
		return types.NewExitCodeError(constants.EXIT_CODE_CATCH_ALL, errors.New("completed with no analysis results"))
	}

	if interactive {
		err = showInteractiveResults(preflightSpecName, output, analyzeResults)
	} else {
		err = showTextResults(format, preflightSpecName, output, analyzeResults)
	}

	if err != nil {
		return err
	}

	exitCode := checkOutcomesToExitCode(analyzeResults)

	if exitCode == 0 {
		return nil
	}

	return types.NewExitCodeError(exitCode, errors.New("preflights failed with warnings or errors"))
}

// Determine if any preflight checks passed vs failed vs warned
// If all checks passed: 0
// If 1 or more checks failed: 3
// If no checks failed, but 1 or more warn: 4
func checkOutcomesToExitCode(analyzeResults []*analyzer.AnalyzeResult) int {
	// Assume pass until they don't
	exitCode := 0

	for _, analyzeResult := range analyzeResults {
		if analyzeResult.IsWarn {
			exitCode = constants.EXIT_CODE_WARN
		} else if analyzeResult.IsFail {
			exitCode = constants.EXIT_CODE_FAIL
			// No need to check further, a fail is a fail
			return exitCode
		}
	}

	return exitCode
}

func collectInteractiveProgress(ctx context.Context, progressCh <-chan interface{}) func() error {
	return func() error {
		spinner := spin.New()
		lastMsg := ""

		errorTxt := color.New(color.FgHiRed)
		infoTxt := color.New(color.FgCyan)

		for {
			select {
			case msg := <-progressCh:
				switch msg := msg.(type) {
				case error:
					errorTxt.Printf("%s\r * %v\n", cursor.ClearEntireLine(), msg)
				case string:
					if lastMsg == msg {
						break
					}
					lastMsg = msg
					infoTxt.Printf("%s\r * %s\n", cursor.ClearEntireLine(), msg)

				}
			case <-time.After(time.Millisecond * 100):
				fmt.Printf("\r  %s %s ", color.CyanString("Running Preflight Checks"), spinner.Next())
			case <-ctx.Done():
				fmt.Printf("\r%s\r", cursor.ClearEntireLine())
				return nil
			}
		}
	}
}

func collectNonInteractiveProgess(ctx context.Context, progressCh <-chan interface{}) func() error {
	return func() error {
		for {
			select {
			case msg := <-progressCh:
				switch msg := msg.(type) {
				case error:
					fmt.Fprintf(os.Stderr, "error - %v\n", msg)
				case string:
					fmt.Fprintf(os.Stderr, "%s\n", msg)
				case CollectProgress:
					fmt.Fprintf(os.Stderr, "%s\n", msg.String())

				}
			case <-ctx.Done():
				return nil
			}
		}
	}
}

func collectInCluster(ctx context.Context, preflightSpec *troubleshootv1beta2.Preflight, progressCh chan interface{}) (*CollectResult, error) {
	v := viper.GetViper()

	restConfig, err := k8sutil.GetRESTConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert kube flags to rest config")
	}

	collectOpts := CollectOpts{
		Namespace:              v.GetString("namespace"),
		IgnorePermissionErrors: v.GetBool("collect-without-permissions"),
		ProgressChan:           progressCh,
		KubernetesRestConfig:   restConfig,
	}

	if v.GetString("since") != "" || v.GetString("since-time") != "" {
		err := parseTimeFlags(v, preflightSpec.Spec.Collectors)
		if err != nil {
			return nil, err
		}
	}

	collectResults, err := CollectWithContext(ctx, collectOpts, preflightSpec)
	if err != nil {
		if collectResults != nil && !collectResults.IsRBACAllowed() {
			if preflightSpec.Spec.UploadResultsTo != "" {
				clusterCollectResults := collectResults.(ClusterCollectResult)
				err := uploadErrors(preflightSpec.Spec.UploadResultsTo, clusterCollectResults.Collectors)
				if err != nil {
					progressCh <- err
				}
			}
		}
		return nil, err
	}

	return &collectResults, nil
}

func collectRemote(ctx context.Context, preflightSpec *troubleshootv1beta2.HostPreflight, progressCh chan interface{}) (*CollectResult, error) {
	v := viper.GetViper()

	restConfig, err := k8sutil.GetRESTConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert kube flags to rest config")
	}

	labelSelector, err := labels.Parse(v.GetString("selector"))
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse selector")
	}

	namespace := v.GetString("namespace")
	if namespace == "" {
		namespace = "default"
	}

	timeout := v.GetDuration("request-timeout")
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	collectOpts := CollectOpts{
		Namespace:              namespace,
		IgnorePermissionErrors: v.GetBool("collect-without-permissions"),
		ProgressChan:           progressCh,
		KubernetesRestConfig:   restConfig,
		Image:                  v.GetString("collector-image"),
		PullPolicy:             v.GetString("collector-pullpolicy"),
		LabelSelector:          labelSelector.String(),
		Timeout:                timeout,
	}

	collectResults, err := CollectRemote(collectOpts, preflightSpec)
	if err != nil {
		return nil, errors.Wrap(err, "failed to collect from remote")
	}

	return &collectResults, nil
}

func collectHost(ctx context.Context, hostPreflightSpec *troubleshootv1beta2.HostPreflight, progressCh chan interface{}) (*CollectResult, error) {
	collectOpts := CollectOpts{
		ProgressChan: progressCh,
	}

	collectResults, err := CollectHost(collectOpts, hostPreflightSpec)
	if err != nil {
		return nil, errors.Wrap(err, "failed to collect from host")
	}

	return &collectResults, nil
}

func parseTimeFlags(v *viper.Viper, collectors []*troubleshootv1beta2.Collect) error {
	var (
		sinceTime time.Time
		err       error
	)
	if v.GetString("since-time") != "" {
		if v.GetString("since") != "" {
			return errors.Errorf("at most one of `sinceTime` or `since` may be specified")
		}
		sinceTime, err = time.Parse(time.RFC3339, v.GetString("since-time"))
		if err != nil {
			return errors.Wrap(err, "unable to parse --since-time flag")
		}
	} else {
		parsedDuration, err := time.ParseDuration(v.GetString("since"))
		if err != nil {
			return errors.Wrap(err, "unable to parse --since flag")
		}
		now := time.Now()
		sinceTime = now.Add(0 - parsedDuration)
	}
	for _, collector := range collectors {
		if collector.Logs != nil {
			if collector.Logs.Limits == nil {
				collector.Logs.Limits = new(troubleshootv1beta2.LogLimits)
			}
			collector.Logs.Limits.SinceTime = metav1.NewTime(sinceTime)
		}
	}
	return nil
}
