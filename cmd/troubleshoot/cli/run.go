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
	"strings"
	"sync"
	"time"

	cursor "github.com/ahmetalpbalkan/go-cursor"
	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/convert"
	"github.com/replicatedhq/troubleshoot/pkg/httputil"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/replicatedhq/troubleshoot/pkg/specs"
	"github.com/replicatedhq/troubleshoot/pkg/supportbundle"
	"github.com/spf13/viper"
	spin "github.com/tj/go-spin"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

func runTroubleshoot(v *viper.Viper, arg []string) error {
	if !v.GetBool("load-cluster-specs") && len(arg) < 1 {
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

	additionalRedactors := &troubleshootv1beta2.Redactor{}

	// Defining `v` below will render using `v` in reference to Viper unusable.
	// Therefore refactoring `v` to `val` will make sure we can still use it.
	for _, val := range arg {

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

		mainBundle = supportbundle.ConcatSpec(mainBundle, supportBundle)

		parsedRedactors, err := supportbundle.ParseRedactorsFromDocs(multidocs)
		if err != nil {
			return errors.Wrap(err, "failed to parse redactors from doc")
		}
		additionalRedactors.Spec.Redactors = append(additionalRedactors.Spec.Redactors, parsedRedactors...)
	}

	if v.GetBool("load-cluster-specs") {
		sbFromCluster, redactorsFromCluster, err := loadClusterSpecs()
		if err != nil {
			return err
		}
		if sbFromCluster == nil {
			return errors.New("no specs found in cluster")
		}
		mainBundle = supportbundle.ConcatSpec(mainBundle, sbFromCluster)
		additionalRedactors.Spec.Redactors = append(additionalRedactors.Spec.Redactors, redactorsFromCluster.Spec.Redactors...)
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

// loadClusterSpecs loads the support bundle and redactor specs from the cluster
// based on troubleshoot.io/kind=support-bundle,troubleshoot.sh/kind=support-bundle label selector. We search for secrets
// and configmaps with the label selector and parse the data as a support bundle. If the
// user does not have sufficient permissions to list & read secrets and configmaps from
// all namespaces, we will fallback to trying each namespace individually, and eventually
// default to the configured kubeconfig namespace.
func loadClusterSpecs() (*troubleshootv1beta2.SupportBundle, *troubleshootv1beta2.Redactor, error) {
	redactors := &troubleshootv1beta2.Redactor{}

	v := viper.GetViper() // It's singleton, so we can use it anywhere

	klog.Info("Discover troubleshoot specs from cluster")

	labelSelector := strings.Join(v.GetStringSlice("selector"), ",")

	parsedSelector, err := labels.Parse(labelSelector)

	if err != nil {
		return nil, nil, errors.Wrap(err, "unable to parse selector")
	}

	config, err := k8sutil.GetRESTConfig()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to convert kube flags to rest config")
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to convert create k8s client")
	}

	// List of namespaces we want to search for secrets and configmaps with support bundle specs
	namespaces := []string{}
	ctx := context.Background()

	if v.GetString("namespace") != "" {
		// Just progress with the namespace provided
		namespaces = []string{v.GetString("namespace")}
	} else {
		// Check if I can read secrets and configmaps in all namespaces
		ican, err := k8sutil.CanIListAndGetAllSecretsAndConfigMaps(ctx, client)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to check if I can read secrets and configmaps")
		}
		klog.V(1).Infof("Can I read any secrets and configmaps: %v", ican)

		if ican {
			// I can read secrets and configmaps in all namespaces
			// No need to iterate over all namespaces
			namespaces = []string{""}
		} else {
			// Get list of all namespaces and try to find specs from each namespace
			nsList, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
			if err != nil {
				if k8serrors.IsForbidden(err) {
					kubeconfig := k8sutil.GetKubeconfig()
					ns, _, err := kubeconfig.Namespace()
					if err != nil {
						return nil, nil, errors.Wrap(err, "failed to get namespace from kubeconfig")
					}
					// If we are not allowed to list namespaces, just use the default namespace
					// configured in the kubeconfig
					namespaces = []string{ns}
				} else {
					return nil, nil, errors.Wrap(err, "failed to list namespaces")
				}
			}

			for _, ns := range nsList.Items {
				namespaces = append(namespaces, ns.Name)
			}
		}
	}

	var bundlesFromCluster []string

	parsedSelectorStrings, err := specs.SplitTroubleshootSecretLabelSelector(client, parsedSelector)
	if err != nil {
		klog.Errorf("failed to parse troubleshoot labels selector %s", err)
	}

	// Search cluster for support bundle specs
	for _, parsedSelectorString := range parsedSelectorStrings {
		for _, ns := range namespaces {
			klog.V(1).Infof("Search support bundle specs from [%q] namespace using %q selector", strings.Join(namespaces, ", "), parsedSelectorString)
			bundlesFromSecrets, err := specs.LoadFromSecretMatchingLabel(client, parsedSelectorString, ns, specs.SupportBundleKey)
			if err != nil {
				if !k8serrors.IsForbidden(err) {
					klog.Errorf("failed to load support bundle spec from secrets: %s", err)
				} else {
					klog.Warningf("Reading secrets from %q namespace forbidden", ns)
				}
			}
			bundlesFromCluster = append(bundlesFromCluster, bundlesFromSecrets...)

			bundlesFromConfigMaps, err := specs.LoadFromConfigMapMatchingLabel(client, parsedSelectorString, ns, specs.SupportBundleKey)
			if err != nil {
				if !k8serrors.IsForbidden(err) {
					klog.Errorf("failed to load support bundle spec from configmap: %s", err)
				} else {
					klog.Warningf("Reading configmaps from %q namespace forbidden", ns)
				}
			}
			bundlesFromCluster = append(bundlesFromCluster, bundlesFromConfigMaps...)
		}
	}

	parsedBundle := &troubleshootv1beta2.SupportBundle{}

	for _, bundle := range bundlesFromCluster {
		multidocs := strings.Split(string(bundle), "\n---\n")
		bundleFromDoc, err := supportbundle.ParseSupportBundleFromDoc([]byte(multidocs[0]))
		if err != nil {
			klog.Errorf("failed to parse support bundle spec:  %s", err)
			continue
		}

		parsedBundle = supportbundle.ConcatSpec(parsedBundle, bundleFromDoc)

		parsedRedactors, err := supportbundle.ParseRedactorsFromDocs(multidocs)
		if err != nil {
			klog.Errorf("failed to parse redactors from doc:  %s", err)
			continue
		}

		redactors.Spec.Redactors = append(redactors.Spec.Redactors, parsedRedactors...)
	}

	var redactorsFromCluster []string

	// Search cluster for redactor specs
	for _, parsedSelectorString := range parsedSelectorStrings {
		for _, ns := range namespaces {
			klog.V(1).Infof("Search redactor specs from [%q] namespace using %q selector", strings.Join(namespaces, ", "), parsedSelectorString)
			redactorsFromSecrets, err := specs.LoadFromSecretMatchingLabel(client, parsedSelectorString, ns, specs.RedactorKey)
			if err != nil {
				if !k8serrors.IsForbidden(err) {
					klog.Errorf("failed to load support bundle spec from secrets: %s", err)
				} else {
					klog.Warningf("Reading secrets from %q namespace forbidden", ns)
				}
			}
			redactorsFromCluster = append(redactorsFromCluster, redactorsFromSecrets...)

			redactorsFromConfigMaps, err := specs.LoadFromConfigMapMatchingLabel(client, parsedSelectorString, ns, specs.RedactorKey)
			if err != nil {
				if !k8serrors.IsForbidden(err) {
					klog.Errorf("failed to load support bundle spec from configmap: %s", err)
				} else {
					klog.Warningf("Reading configmaps from %q namespace forbidden", ns)
				}
			}
			redactorsFromCluster = append(redactorsFromCluster, redactorsFromConfigMaps...)
		}
	}

	for _, redactor := range redactorsFromCluster {
		multidocs := strings.Split(string(redactor), "\n---\n")
		parsedRedactors, err := supportbundle.ParseRedactorsFromDocs(multidocs)
		if err != nil {
			klog.Errorf("failed to parse redactors from doc:  %s", err)
		}

		redactors.Spec.Redactors = append(redactors.Spec.Redactors, parsedRedactors...)
	}

	return parsedBundle, redactors, nil
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
