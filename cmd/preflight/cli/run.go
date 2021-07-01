package cli

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	cursor "github.com/ahmetalpbalkan/go-cursor"
	"github.com/fatih/color"
	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/cmd/util"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	troubleshootclientsetscheme "github.com/replicatedhq/troubleshoot/pkg/client/troubleshootclientset/scheme"
	"github.com/replicatedhq/troubleshoot/pkg/docrewrite"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/replicatedhq/troubleshoot/pkg/preflight"
	"github.com/replicatedhq/troubleshoot/pkg/specs"
	"github.com/spf13/viper"
	spin "github.com/tj/go-spin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
)

func runPreflights(v *viper.Viper, arg string) error {
	fmt.Print(cursor.Hide())
	defer fmt.Print(cursor.Show())

	go func() {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt)
		<-signalChan
		fmt.Print(cursor.Show())
		os.Exit(0)
	}()

	var preflightContent []byte
	var err error
	if strings.HasPrefix(arg, "secret/") {
		// format secret/namespace-name/secret-name
		pathParts := strings.Split(arg, "/")
		if len(pathParts) != 3 {
			return errors.Errorf("path %s must have 3 components", arg)
		}

		spec, err := specs.LoadFromSecret(pathParts[1], pathParts[2], "preflight-spec")
		if err != nil {
			return errors.Wrap(err, "failed to get spec from secret")
		}

		preflightContent = spec
	} else if _, err = os.Stat(arg); err == nil {
		b, err := ioutil.ReadFile(arg)
		if err != nil {
			return err
		}

		preflightContent = b
	} else {
		if !util.IsURL(arg) {
			return fmt.Errorf("%s is not a URL and was not found (err %s)", arg, err)
		}

		req, err := http.NewRequest("GET", arg, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", "Replicated_Preflight/v1beta2")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		preflightContent = body
	}

	preflightContent, err = docrewrite.ConvertToV1Beta2(preflightContent)
	if err != nil {
		return errors.Wrap(err, "failed to convert to v1beta2")
	}

	troubleshootclientsetscheme.AddToScheme(scheme.Scheme)
	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode([]byte(preflightContent), nil, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to parse %s", arg)
	}

	var collectResults preflight.CollectResult
	preflightSpecName := ""
	finishedCh := make(chan bool, 1)
	progressCh := make(chan interface{}, 0) // non-zero buffer will result in missed messages

	if v.GetBool("interactive") {
		s := spin.New()
		go func() {
			lastMsg := ""
			for {
				select {
				case msg, ok := <-progressCh:
					if !ok {
						continue
					}
					switch msg := msg.(type) {
					case error:
						c := color.New(color.FgHiRed)
						c.Println(fmt.Sprintf("%s\r * %v", cursor.ClearEntireLine(), msg))
					case string:
						if lastMsg == msg {
							break
						}
						lastMsg = msg
						c := color.New(color.FgCyan)
						c.Println(fmt.Sprintf("%s\r * %s", cursor.ClearEntireLine(), msg))
					}
				case <-time.After(time.Millisecond * 100):
					fmt.Printf("\r  \033[36mRunning Preflight checks\033[m %s ", s.Next())
				case <-finishedCh:
					fmt.Printf("\r%s\r", cursor.ClearEntireLine())
					return
				}
			}
		}()
	} else {
		// make sure we don't block any senders
		go func() {
			for {
				select {
				case _, ok := <-progressCh:
					if !ok {
						return
					}
				case <-finishedCh:
					return
				}
			}
		}()
	}

	defer func() {
		close(finishedCh)
		close(progressCh)
	}()

	if preflightSpec, ok := obj.(*troubleshootv1beta2.Preflight); ok {
		r, err := collectInCluster(preflightSpec, finishedCh, progressCh)
		if err != nil {
			return errors.Wrap(err, "failed to collect in cluster")
		}
		collectResults = *r
		preflightSpecName = preflightSpec.Name
	} else if hostPreflightSpec, ok := obj.(*troubleshootv1beta2.HostPreflight); ok {
		r, err := collectHost(hostPreflightSpec, finishedCh, progressCh)
		if err != nil {
			return errors.Wrap(err, "failed to collect from host")
		}
		collectResults = *r
		preflightSpecName = hostPreflightSpec.Name
	}

	if collectResults == nil {
		return errors.New("no results")
	}

	analyzeResults := collectResults.Analyze()

	if preflightSpec, ok := obj.(*troubleshootv1beta2.Preflight); ok {
		if preflightSpec.Spec.UploadResultsTo != "" {
			err := uploadResults(preflightSpec.Spec.UploadResultsTo, analyzeResults)
			if err != nil {
				progressCh <- err
			}
		}
	}

	finishedCh <- true

	if v.GetBool("interactive") {
		if len(analyzeResults) == 0 {
			return errors.New("no data has been collected")
		}
		return showInteractiveResults(preflightSpecName, analyzeResults)
	}

	return showStdoutResults(v.GetString("format"), preflightSpecName, analyzeResults)
}

func collectInCluster(preflightSpec *troubleshootv1beta2.Preflight, finishedCh chan bool, progressCh chan interface{}) (*preflight.CollectResult, error) {
	v := viper.GetViper()

	restConfig, err := k8sutil.GetRESTConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert kube flags to rest config")
	}

	collectOpts := preflight.CollectOpts{
		Namespace:              v.GetString("namespace"),
		IgnorePermissionErrors: v.GetBool("collect-without-permissions"),
		ProgressChan:           progressCh,
		KubernetesRestConfig:   restConfig,
	}

	if v.GetString("since") != "" || v.GetString("since-time") != "" {
		err := parseTimeFlags(v, progressCh, preflightSpec.Spec.Collectors)
		if err != nil {
			return nil, err
		}
	}

	collectResults, err := preflight.Collect(collectOpts, preflightSpec)
	if err != nil {
		if !collectResults.IsRBACAllowed() {
			if preflightSpec.Spec.UploadResultsTo != "" {
				clusterCollectResults := collectResults.(preflight.ClusterCollectResult)
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

func collectHost(hostPreflightSpec *troubleshootv1beta2.HostPreflight, finishedCh chan bool, progressCh chan interface{}) (*preflight.CollectResult, error) {
	collectOpts := preflight.CollectOpts{
		ProgressChan: progressCh,
	}

	collectResults, err := preflight.CollectHost(collectOpts, hostPreflightSpec)
	if err != nil {
		return nil, errors.Wrap(err, "failed to collect from host")
	}

	return &collectResults, nil
}

func parseTimeFlags(v *viper.Viper, progressChan chan interface{}, collectors []*troubleshootv1beta2.Collect) error {
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
