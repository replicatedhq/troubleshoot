package cli

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"time"

	cursor "github.com/ahmetalpbalkan/go-cursor"
	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/internal/specs"
	"github.com/replicatedhq/troubleshoot/internal/util"
	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"github.com/replicatedhq/troubleshoot/pkg/convert"
	"github.com/replicatedhq/troubleshoot/pkg/httputil"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/replicatedhq/troubleshoot/pkg/loader"
	"github.com/replicatedhq/troubleshoot/pkg/supportbundle"
	"github.com/replicatedhq/troubleshoot/pkg/types"
	"github.com/spf13/viper"
	spin "github.com/tj/go-spin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

func runTroubleshoot(v *viper.Viper, args []string) error {
	ctx := context.Background()
	if !v.GetBool("load-cluster-specs") && len(args) < 1 {
		return errors.New("flag load-cluster-specs must be set if no specs are provided on the command line")
	}

	restConfig, err := k8sutil.GetRESTConfig()
	if err != nil {
		return errors.Wrap(err, "failed to convert kube flags to rest config")
	}

	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return errors.Wrap(err, "failed to create kubernetes client")
	}

	mainBundle, additionalRedactors, err := loadSpecs(ctx, args, client)
	if err != nil {
		return err
	}

	// For --dry-run, we want to print the yaml and exit
	if v.GetBool("dry-run") {
		k := loader.TroubleshootKinds{
			SupportBundlesV1Beta2: []troubleshootv1beta2.SupportBundle{*mainBundle},
		}
		// If we have redactors, add them to the temp kinds object
		if len(additionalRedactors.Spec.Redactors) > 0 {
			k.RedactorsV1Beta2 = []troubleshootv1beta2.Redactor{*additionalRedactors}
		}

		out, err := k.ToYaml()
		if err != nil {
			return types.NewExitCodeError(constants.EXIT_CODE_CATCH_ALL, errors.Wrap(err, "failed to convert specs to yaml"))
		}
		fmt.Printf("%s", out)
		return nil
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

	var wg sync.WaitGroup
	collectorCB := func(c chan interface{}, msg string) { c <- msg }
	progressChan := make(chan interface{})
	isProgressChanClosed := false
	defer func() {
		if !isProgressChanClosed {
			close(progressChan)
		}
		wg.Wait()
	}()

	if !interactive {
		// TODO (dans): custom warning handler to capture warning in `analysisOutput`
		restConfig.WarningHandler = rest.NoWarnings{}

		// TODO (dans): maybe log to file
		wg.Add(1)
		go func() {
			defer wg.Done()
			for msg := range progressChan {
				klog.Infof("Collecting support bundle: %v", msg)
			}
		}()
	} else {
		s := spin.New()
		wg.Add(1)
		go func() {
			defer wg.Done()
			currentDir := ""
			for {
				select {
				case msg, ok := <-progressChan:
					if !ok {
						fmt.Printf("\r%s\r", cursor.ClearEntireLine())
						return
					}
					switch msg := msg.(type) {
					case error:
						c := color.New(color.FgHiRed)
						c.Println(fmt.Sprintf("%s\r * %v", cursor.ClearEntireLine(), msg))
					case string:
						currentDir = filepath.Base(msg)
					}
				case <-time.After(time.Millisecond * 100):
					if currentDir == "" {
						fmt.Printf("\r%s \033[36mCollecting support bundle\033[m %s", cursor.ClearEntireLine(), s.Next())
					} else {
						fmt.Printf("\r%s \033[36mCollecting support bundle\033[m %s %s", cursor.ClearEntireLine(), s.Next(), currentDir)
					}
				}
			}
		}()
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

	close(progressChan) // this removes the spinner in interactive mode
	isProgressChanClosed = true

	if len(response.AnalyzerResults) > 0 {
		if interactive {
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

		fmt.Printf("\n%s\n", response.ArchivePath)
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

// loadSupportBundleSpecsFromURIs loads support bundle specs from URIs
func loadSupportBundleSpecsFromURIs(ctx context.Context, kinds *loader.TroubleshootKinds) (*loader.TroubleshootKinds, error) {
	remoteRawSpecs := []string{}
	for _, s := range kinds.SupportBundlesV1Beta2 {
		if s.Spec.Uri != "" && util.IsURL(s.Spec.Uri) {
			// We are using LoadSupportBundleSpec function here since it handles prompting
			// users to accept insecure connections
			// There is an opportunity to refactor this code in favour of the Loader APIs
			// TODO: Pass ctx to LoadSupportBundleSpec
			rawSpec, err := supportbundle.LoadSupportBundleSpec(s.Spec.Uri)
			if err != nil {
				// In the event a spec can't be loaded, we'll just skip it and print a warning
				klog.Warningf("unable to load support bundle from URI: %q: %v", s.Spec.Uri, err)
				continue
			}
			remoteRawSpecs = append(remoteRawSpecs, string(rawSpec))
		}
	}

	if len(remoteRawSpecs) == 0 {
		return kinds, nil
	}

	return loader.LoadSpecs(ctx, loader.LoadOptions{
		RawSpecs: remoteRawSpecs,
	})
}

func loadSpecs(ctx context.Context, args []string, client kubernetes.Interface) (*troubleshootv1beta2.SupportBundle, *troubleshootv1beta2.Redactor, error) {
	// Append redactor uris to the args
	allArgs := append(args, viper.GetStringSlice("redactors")...)
	kinds, err := specs.LoadFromCLIArgs(ctx, client, allArgs, viper.GetViper())
	if err != nil {
		return nil, nil, err
	}

	// Load additional specs from support bundle URIs
	if !viper.GetBool("no-uri") {
		moreKinds, err := loadSupportBundleSpecsFromURIs(ctx, kinds)
		if err != nil {
			klog.Warningf("unable to load support bundles from URIs: %v", err)
		} else {
			kinds.Add(moreKinds)
		}
	}

	// Check if we have any collectors to run in the troubleshoot specs
	// TODO: Do we use the RemoteCollectors anymore?
	if len(kinds.CollectorsV1Beta2) == 0 &&
		len(kinds.HostCollectorsV1Beta2) == 0 &&
		len(kinds.SupportBundlesV1Beta2) == 0 {
		return nil, nil, errors.New("no collectors specified to run")
	}

	// Merge specs
	// We need to add the default type information to the support bundle spec
	// since by default these fields would be empty
	mainBundle := &troubleshootv1beta2.SupportBundle{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "troubleshoot.sh/v1beta2",
			Kind:       "SupportBundle",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "merged-support-bundle-spec",
		},
	}
	for _, sb := range kinds.SupportBundlesV1Beta2 {
		sb := sb
		mainBundle = supportbundle.ConcatSpec(mainBundle, &sb)
	}

	for _, c := range kinds.CollectorsV1Beta2 {
		mainBundle.Spec.Collectors = append(mainBundle.Spec.Collectors, c.Spec.Collectors...)
	}

	for _, hc := range kinds.HostCollectorsV1Beta2 {
		mainBundle.Spec.HostCollectors = append(mainBundle.Spec.HostCollectors, hc.Spec.Collectors...)
	}

	// Ensure cluster info and cluster resources collectors are in the merged spec
	// We need to add them here so when we --dry-run, these collectors are included.
	// supportbundle.runCollectors duplicates this bit. We'll need to refactor it out later
	// when its clearer what other code depends on this logic e.g KOTS
	mainBundle.Spec.Collectors = collect.EnsureCollectorInList(
		mainBundle.Spec.Collectors,
		troubleshootv1beta2.Collect{ClusterInfo: &troubleshootv1beta2.ClusterInfo{}},
	)
	mainBundle.Spec.Collectors = collect.EnsureCollectorInList(
		mainBundle.Spec.Collectors,
		troubleshootv1beta2.Collect{ClusterResources: &troubleshootv1beta2.ClusterResources{}},
	)

	additionalRedactors := &troubleshootv1beta2.Redactor{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "troubleshoot.sh/v1beta2",
			Kind:       "Redactor",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "merged-redactors-spec",
		},
	}
	for _, r := range kinds.RedactorsV1Beta2 {
		additionalRedactors.Spec.Redactors = append(additionalRedactors.Spec.Redactors, r.Spec.Redactors...)
	}

	return mainBundle, additionalRedactors, nil
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
