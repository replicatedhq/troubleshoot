package preflight

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	cursor "github.com/ahmetalpbalkan/go-cursor"
	"github.com/fatih/color"
	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/cmd/util"
	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	troubleshootclientsetscheme "github.com/replicatedhq/troubleshoot/pkg/client/troubleshootclientset/scheme"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"github.com/replicatedhq/troubleshoot/pkg/docrewrite"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/replicatedhq/troubleshoot/pkg/oci"
	"github.com/replicatedhq/troubleshoot/pkg/specs"
	"github.com/spf13/viper"
	spin "github.com/tj/go-spin"
	"go.opentelemetry.io/otel"
	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/scheme"
)

func RunPreflights(interactive bool, output string, format string, args []string) error {
	ctx, root := otel.Tracer(constants.LIB_TRACER_NAME).Start(context.Background(), "troubleshoot-root")
	defer root.End()

	if interactive {
		fmt.Print(cursor.Hide())
		defer fmt.Print(cursor.Show())
	}

	go func() {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt)
		<-signalChan
		os.Exit(0)
	}()

	var preflightContent []byte
	var preflightSpec *troubleshootv1beta2.Preflight
	var hostPreflightSpec *troubleshootv1beta2.HostPreflight
	var uploadResultSpecs []*troubleshootv1beta2.Preflight
	var err error

	for _, v := range args {
		if strings.HasPrefix(v, "secret/") {
			// format secret/namespace-name/secret-name
			pathParts := strings.Split(v, "/")
			if len(pathParts) != 3 {
				return errors.Errorf("path %s must have 3 components", v)
			}

			spec, err := specs.LoadFromSecret(pathParts[1], pathParts[2], "preflight-spec")
			if err != nil {
				return errors.Wrap(err, "failed to get spec from secret")
			}

			preflightContent = spec
		} else if _, err = os.Stat(v); err == nil {
			b, err := os.ReadFile(v)
			if err != nil {
				return err
			}

			preflightContent = b
		} else {
			u, err := url.Parse(v)
			if err != nil {
				return err
			}

			if u.Scheme == "oci" {
				content, err := oci.PullPreflightFromOCI(v)
				if err != nil {
					if err == oci.ErrNoRelease {
						return errors.Errorf("no release found for %s.\nCheck the oci:// uri for errors or contact the application vendor for support.", v)
					}

					return err
				}

				preflightContent = content
			} else {
				if !util.IsURL(v) {
					return fmt.Errorf("%s is not a URL and was not found (err %s)", v, err)
				}

				req, err := http.NewRequest("GET", v, nil)
				if err != nil {
					return err
				}
				req.Header.Set("User-Agent", "Replicated_Preflight/v1beta2")
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					return err
				}
				defer resp.Body.Close()

				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return err
				}

				preflightContent = body
			}
		}

		preflightContent, err = docrewrite.ConvertToV1Beta2(preflightContent)
		if err != nil {
			return errors.Wrap(err, "failed to convert to v1beta2")
		}

		troubleshootclientsetscheme.AddToScheme(scheme.Scheme)
		decode := scheme.Codecs.UniversalDeserializer().Decode
		obj, _, err := decode([]byte(preflightContent), nil, nil)
		if err != nil {
			return errors.Wrapf(err, "failed to parse %s", v)
		}

		if spec, ok := obj.(*troubleshootv1beta2.Preflight); ok {
			if spec.Spec.UploadResultsTo == "" {
				preflightSpec = ConcatPreflightSpec(preflightSpec, spec)
			} else {
				uploadResultSpecs = append(uploadResultSpecs, spec)
			}
		} else if spec, ok := obj.(*troubleshootv1beta2.HostPreflight); ok {
			hostPreflightSpec = ConcatHostPreflightSpec(hostPreflightSpec, spec)
		}
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

	if preflightSpec != nil {
		r, err := collectInCluster(ctx, preflightSpec, progressCh)
		if err != nil {
			return errors.Wrap(err, "failed to collect in cluster")
		}
		collectResults = append(collectResults, *r)
		preflightSpecName = preflightSpec.Name
	}
	if uploadResultSpecs != nil {
		for _, spec := range uploadResultSpecs {
			r, err := collectInCluster(ctx, spec, progressCh)
			if err != nil {
				return errors.Wrap(err, "failed to collect in cluster")
			}
			uploadResultsMap[spec.Spec.UploadResultsTo] = append(uploadResultsMap[spec.Spec.UploadResultsTo], *r)
			uploadCollectResults = append(collectResults, *r)
			preflightSpecName = spec.Name
		}
	}
	if hostPreflightSpec != nil {
		if len(hostPreflightSpec.Spec.Collectors) > 0 {
			r, err := collectHost(ctx, hostPreflightSpec, progressCh)
			if err != nil {
				return errors.Wrap(err, "failed to collect from host")
			}
			collectResults = append(collectResults, *r)
		}
		if len(hostPreflightSpec.Spec.RemoteCollectors) > 0 {
			r, err := collectRemote(ctx, hostPreflightSpec, progressCh)
			if err != nil {
				return errors.Wrap(err, "failed to collect remotely")
			}
			collectResults = append(collectResults, *r)
		}
		preflightSpecName = hostPreflightSpec.Name
	}

	if collectResults == nil && uploadCollectResults == nil {
		return errors.New("no results")
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

	if uploadAnalyzeResultsMap != nil {
		for k, v := range uploadAnalyzeResultsMap {
			err := uploadResults(k, v)
			if err != nil {
				progressCh <- err
			}
		}
	}

	stopProgressCollection()
	progressCollection.Wait()

	if interactive {
		if len(analyzeResults) == 0 {
			return errors.New("no data has been collected")
		}
		return showInteractiveResults(preflightSpecName, output, analyzeResults)
	}

	return showTextResults(format, preflightSpecName, output, analyzeResults)
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
