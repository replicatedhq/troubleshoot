package cli

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	cursor "github.com/ahmetalpbalkan/go-cursor"
	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/client/troubleshootclientset/scheme"
	troubleshootclientsetscheme "github.com/replicatedhq/troubleshoot/pkg/client/troubleshootclientset/scheme"
	"github.com/replicatedhq/troubleshoot/pkg/convert"
	"github.com/replicatedhq/troubleshoot/pkg/httputil"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	"github.com/replicatedhq/troubleshoot/pkg/specs"
	"github.com/replicatedhq/troubleshoot/pkg/supportbundle"
	"github.com/spf13/viper"
	spin "github.com/tj/go-spin"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func runTroubleshoot(v *viper.Viper, arg []string) error {
	if v.GetBool("load-cluster-specs") == false && len(arg) < 1 {
		return errors.New("flag load-cluster-specs must be set if no specs are provided on the command line")
	}

	interactive := v.GetBool("interactive") && isatty.IsTerminal(os.Stdout.Fd())

	if interactive {
		fmt.Print(cursor.Hide())
		defer fmt.Print(cursor.Show())
	}

	go func() {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt)
		<-signalChan
		if interactive {
			fmt.Print(cursor.Show())
		}
		os.Exit(0)
	}()

	restConfig, err := k8sutil.GetRESTConfig()
	if err != nil {
		return errors.Wrap(err, "failed to convert kube flags to rest config")
	}

	var sinceTime *time.Time
	if v.GetString("since-time") != "" || v.GetString("since") != "" {
		sinceTime, err = parseTimeFlags(v)
		if err != nil {
			return errors.Wrap(err, "failed parse since time")
		}
	}

	if v.GetBool("allow-insecure-connections") || v.GetBool("insecure-skip-tls-verify") {
		httputil.AddTransport(&http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		})
	}

	var mainBundle *troubleshootv1beta2.SupportBundle

	troubleshootclientsetscheme.AddToScheme(scheme.Scheme)
	additionalRedactors := &troubleshootv1beta2.Redactor{}

	// Defining `v` below will render using `v` in reference to Viper unusable.
	// Therefore refactoring `v` to `val` will make sure we can still use it.
	for i, val := range arg {

		collectorContent, err := supportbundle.LoadSupportBundleSpec(val)
		if err != nil {
			return errors.Wrap(err, "failed to load support bundle spec")
		}
		multidocs := strings.Split(string(collectorContent), "\n---\n")
		// Referencing `ParseSupportBundle with a secondary arg of `no-uri`
		// Will make sure we can enable or disable the use of the `Spec.uri` field for an upstream spec.
		// This change will not have an impact on KOTS' usage of `ParseSupportBundle`
		// As Kots uses `load.go` directly.
		supportBundle, err := supportbundle.ParseSupportBundle([]byte(multidocs[0]), !v.GetBool("no-uri"))
		if err != nil {
			return errors.Wrap(err, "failed to parse support bundle spec")
		}

		if i == 0 {
			mainBundle = supportBundle
		} else {
			mainBundle = supportbundle.ConcatSpec(mainBundle, supportBundle)
		}

		parsedRedactors, err := supportbundle.ParseRedactorsFromDocs(multidocs)
		if err != nil {
			return errors.Wrap(err, "failed to parse redactors from doc")
		}
		additionalRedactors.Spec.Redactors = append(additionalRedactors.Spec.Redactors, parsedRedactors...)
	}

	if v.GetBool("load-cluster-specs") {
		labelSelector := strings.Join(v.GetStringSlice("selector"), ",")

		parsedSelector, err := labels.Parse(labelSelector)
		if err != nil {
			return errors.Wrap(err, "unable to parse selector")
		}

		namespace := ""
		if v.GetString("namespace") != "" {
			namespace = v.GetString("namespace")
		}

		config, err := k8sutil.GetRESTConfig()
		if err != nil {
			return errors.Wrap(err, "failed to convert kube flags to rest config")
		}

		client, err := kubernetes.NewForConfig(config)
		if err != nil {
			return errors.Wrap(err, "failed to convert create k8s client")
		}

		var bundlesFromCluster []string

		// Search cluster for Troubleshoot objects in cluster
		bundlesFromSecrets, err := specs.LoadFromSecretMatchingLabel(client, parsedSelector.String(), namespace, specs.SupportBundleKey)
		if err != nil {
			logger.Printf("failed to load support bundle spec from secrets: %s", err)
		}
		bundlesFromCluster = append(bundlesFromCluster, bundlesFromSecrets...)

		bundlesFromConfigMaps, err := specs.LoadFromConfigMapMatchingLabel(client, parsedSelector.String(), namespace, specs.SupportBundleKey)
		if err != nil {
			logger.Printf("failed to load support bundle spec from secrets: %s", err)
		}
		bundlesFromCluster = append(bundlesFromCluster, bundlesFromConfigMaps...)

		for _, bundle := range bundlesFromCluster {
			multidocs := strings.Split(string(bundle), "\n---\n")
			parsedBundleFromSecret, err := supportbundle.ParseSupportBundleFromDoc([]byte(multidocs[0]))
			if err != nil {
				logger.Printf("failed to parse support bundle spec:  %s", err)
				continue
			}

			if mainBundle == nil {
				mainBundle = parsedBundleFromSecret
			} else {
				mainBundle = supportbundle.ConcatSpec(mainBundle, parsedBundleFromSecret)
			}

			parsedRedactors, err := supportbundle.ParseRedactorsFromDocs(multidocs)
			if err != nil {
				logger.Printf("failed to parse redactors from doc:  %s", err)
				continue
			}

			additionalRedactors.Spec.Redactors = append(additionalRedactors.Spec.Redactors, parsedRedactors...)
		}

		var redactorsFromCluster []string

		// Search cluster for Troubleshoot objects in ConfigMaps
		redactorsFromSecrets, err := specs.LoadFromSecretMatchingLabel(client, parsedSelector.String(), namespace, specs.RedactorKey)
		if err != nil {
			logger.Printf("failed to load redactor specs from config maps: %s", err)
		}
		redactorsFromCluster = append(redactorsFromCluster, redactorsFromSecrets...)

		redactorsFromConfigMaps, err := specs.LoadFromConfigMapMatchingLabel(client, parsedSelector.String(), namespace, specs.RedactorKey)
		if err != nil {
			logger.Printf("failed to load redactor specs from config maps: %s", err)
		}
		redactorsFromCluster = append(redactorsFromCluster, redactorsFromConfigMaps...)

		for _, redactor := range redactorsFromCluster {
			multidocs := strings.Split(string(redactor), "\n---\n")
			parsedRedactors, err := supportbundle.ParseRedactorsFromDocs(multidocs)
			if err != nil {
				logger.Printf("failed to parse redactors from doc:  %s", err)
			}

			additionalRedactors.Spec.Redactors = append(additionalRedactors.Spec.Redactors, parsedRedactors...)
		}

		if mainBundle == nil {
			return errors.New("no specs found in cluster")
		}
	}

	if mainBundle == nil {
		return errors.New("no support bundle specs provided to run")
	} else if mainBundle.Spec.Collectors == nil && mainBundle.Spec.HostCollectors == nil {
		return errors.New("no collectors specified in support bundle")
	}

	redactors, err := supportbundle.GetRedactorsFromURIs(v.GetStringSlice("redactors"))
	if err != nil {
		return errors.Wrap(err, "failed to get redactors")
	}
	additionalRedactors.Spec.Redactors = append(additionalRedactors.Spec.Redactors, redactors...)

	var collectorCB func(chan interface{}, string)
	progressChan := make(chan interface{}) // non-zero buffer can result in missed messages
	finishedCh := make(chan bool, 1)
	isFinishedChClosed := false

	if !interactive {
		// TODO (dans): custom warning handler to capture warning in `analysisOutput`
		restConfig.WarningHandler = rest.NoWarnings{}
		collectorCB = func(ch chan interface{}, name string) {
			return
		}

		// TODO (dans): maybe log to file
		go func() {
			for {
				select {
				case _ = <-progressChan:
					// do nothing
				}
			}
		}()
	} else {
		s := spin.New()
		go func() {
			currentDir := ""
			for {
				select {
				case msg := <-progressChan:
					switch msg := msg.(type) {
					case error:
						c := color.New(color.FgHiRed)
						c.Println(fmt.Sprintf("%s\r * %v", cursor.ClearEntireLine(), msg))
					case string:
						currentDir = filepath.Base(msg)
					}
				case <-finishedCh:
					fmt.Printf("\r%s\r", cursor.ClearEntireLine())
					return
				case <-time.After(time.Millisecond * 100):
					if currentDir == "" {
						fmt.Printf("\r%s \033[36mCollecting support bundle\033[m %s", cursor.ClearEntireLine(), s.Next())
					} else {
						fmt.Printf("\r%s \033[36mCollecting support bundle\033[m %s %s", cursor.ClearEntireLine(), s.Next(), currentDir)
					}
				}
			}
		}()
		defer func() {
			if !isFinishedChClosed {
				close(finishedCh)
			}
		}()

		collectorCB = func(c chan interface{}, msg string) {
			c <- fmt.Sprintf("%s", msg)
		}

	}

	createOpts := supportbundle.SupportBundleCreateOpts{
		CollectorProgressCallback: collectorCB,
		CollectWithoutPermissions: v.GetBool("collect-without-permissions"),
		KubernetesRestConfig:      restConfig,
		Namespace:                 v.GetString("namespace"),
		ProgressChan:              progressChan,
		SinceTime:                 sinceTime,
		OutputPath:                v.GetString("output"),
		Redact:                    v.GetBool("redact"),
		FromCLI:                   true,
	}

	nonInteractiveOutput := analysisOutput{}

	if interactive {
		c := color.New()
		c.Println(fmt.Sprintf("\r%s\r", cursor.ClearEntireLine()))
	}

	response, err := supportbundle.CollectSupportBundleFromSpec(&mainBundle.Spec, additionalRedactors, createOpts)
	if err != nil {
		return errors.Wrap(err, "failed to run collect and analyze process")
	}
	if len(response.AnalyzerResults) > 0 {
		if interactive {
			close(finishedCh) // this removes the spinner
			isFinishedChClosed = true

			if err := showInteractiveResults(mainBundle.Name, response.AnalyzerResults); err != nil {
				interactive = false
			}
		} else {
			nonInteractiveOutput.Analysis = response.AnalyzerResults
		}
	}

	if !response.FileUploaded {
		if appName := mainBundle.Labels["applicationName"]; appName != "" {
			f := `A support bundle for %s has been created in this directory
named %s. Please upload it on the Troubleshoot page of
the %s Admin Console to begin analysis.`
			fmt.Printf(f, appName, response.ArchivePath, appName)
			return nil
		}

		if !interactive {
			nonInteractiveOutput.ArchivePath = response.ArchivePath
			output, err := nonInteractiveOutput.FormattedAnalysisOutput()
			if err != nil {
				return errors.Wrap(err, "failed to format non-interactive output")
			}
			fmt.Println(output)
			return nil
		}

		fmt.Printf("%s\n", response.ArchivePath)
		return nil
	}

	if interactive {
		fmt.Printf("\r%s\r", cursor.ClearEntireLine())
	}
	if response.FileUploaded {
		fmt.Printf("A support bundle has been created and uploaded to your cluster for analysis. Please visit the Troubleshoot page to continue.\n")
		fmt.Printf("A copy of this support bundle was written to the current directory, named %q\n", response.ArchivePath)
	} else {
		fmt.Printf("A support bundle has been created in the current directory named %q\n", response.ArchivePath)
	}
	return nil
}

func getExpectedContentType(uploadURL string) string {
	parsedURL, err := url.Parse(uploadURL)
	if err != nil {
		return ""
	}
	return parsedURL.Query().Get("Content-Type")
}

func parseTimeFlags(v *viper.Viper) (*time.Time, error) {
	var (
		sinceTime time.Time
		err       error
	)
	if v.GetString("since-time") != "" {
		if v.GetString("since") != "" {
			return nil, errors.Errorf("at most one of `sinceTime` or `since` may be specified")
		}
		sinceTime, err = time.Parse(time.RFC3339, v.GetString("since-time"))
		if err != nil {
			return nil, errors.Wrap(err, "unable to parse --since-time flag")
		}
	} else {
		parsedDuration, err := time.ParseDuration(v.GetString("since"))
		if err != nil {
			return nil, errors.Wrap(err, "unable to parse --since flag")
		}
		now := time.Now()
		sinceTime = now.Add(0 - parsedDuration)
	}

	return &sinceTime, nil
}

func shouldRetryRequest(err error) bool {
	if strings.Contains(err.Error(), "x509") && canTryInsecure() {
		httputil.AddTransport(&http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		})
		return true
	}
	return false
}

func canTryInsecure() bool {
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		return false
	}
	prompt := promptui.Prompt{
		Label:     "Connection appears to be insecure. Would you like to attempt to create a support bundle anyway?",
		IsConfirm: true,
	}

	_, err := prompt.Run()
	return err == nil
}

type analysisOutput struct {
	Analysis    []*analyzer.AnalyzeResult
	ArchivePath string
}

func (a *analysisOutput) FormattedAnalysisOutput() (outputJson string, err error) {
	type convertedOutput struct {
		ConvertedAnalysis []*convert.Result `json:"analyzerResults"`
		ArchivePath       string            `json:"archivePath"`
	}

	converted := convert.FromAnalyzerResult(a.Analysis)

	o := convertedOutput{
		ConvertedAnalysis: converted,
		ArchivePath:       a.ArchivePath,
	}

	formatted, err := json.MarshalIndent(o, "", "    ")
	if err != nil {
		return "", fmt.Errorf("\r * Failed to format analysis: %v\n", err)
	}
	return string(formatted), nil
}
