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
	"reflect"
	"strings"
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
	"github.com/replicatedhq/troubleshoot/pkg/redact"
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

	// Validate auto-discovery flags
	if err := ValidateAutoDiscoveryFlags(v); err != nil {
		return errors.Wrap(err, "invalid auto-discovery configuration")
	}

	// Validate tokenization flags
	if err := ValidateTokenizationFlags(v); err != nil {
		return errors.Wrap(err, "invalid tokenization configuration")
	}
	// Apply auto-discovery if enabled
	autoConfig := GetAutoDiscoveryConfig(v)
	if autoConfig.Enabled {
		mode := GetAutoDiscoveryMode(args, autoConfig.Enabled)
		if !v.GetBool("quiet") {
			PrintAutoDiscoveryInfo(autoConfig, mode)
		}

		// Apply auto-discovery to the main bundle
		namespace := v.GetString("namespace")
		if err := ApplyAutoDiscovery(ctx, client, restConfig, mainBundle, autoConfig, namespace); err != nil {
			return errors.Wrap(err, "auto-discovery failed")
		}
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

	if interactive {
		c := color.New()
		c.Println(fmt.Sprintf("\r%s\r", cursor.ClearEntireLine()))
	}

	if interactive {
		if len(mainBundle.Spec.HostCollectors) > 0 && !util.IsRunningAsRoot() && !mainBundle.Spec.RunHostCollectorsInPod {
			fmt.Print(cursor.Show())
			if util.PromptYesNo(util.HOST_COLLECTORS_RUN_AS_ROOT_PROMPT) {
				fmt.Println("Exiting...")
				return nil
			}
			fmt.Print(cursor.Hide())
		}
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
		RunHostCollectorsInPod:    mainBundle.Spec.RunHostCollectorsInPod,

		// Phase 4: Tokenization options
		Tokenize:            v.GetBool("tokenize"),
		RedactionMapPath:    v.GetString("redaction-map"),
		EncryptRedactionMap: v.GetBool("encrypt-redaction-map"),
		TokenPrefix:         v.GetString("token-prefix"),
		VerifyTokenization:  v.GetBool("verify-tokenization"),
		BundleID:            v.GetString("bundle-id"),
		TokenizationStats:   v.GetBool("tokenization-stats"),
	}

	nonInteractiveOutput := analysisOutput{}

	response, err := supportbundle.CollectSupportBundleFromSpec(&mainBundle.Spec, additionalRedactors, createOpts)
	if err != nil {
		return errors.Wrap(err, "failed to run collect and analyze process")
	}

	close(progressChan) // this removes the spinner in interactive mode
	isProgressChanClosed = true

	if len(response.AnalyzerResults) > 0 {
		if interactive {
			if err := showInteractiveResults(mainBundle.Name, response.AnalyzerResults, response.ArchivePath); err != nil {
				interactive = false
			}
		} else {
			nonInteractiveOutput.Analysis = response.AnalyzerResults
		}
	}

	// Attempt auto-upload before any early returns
	if v.GetBool("auto-upload") && !response.FileUploaded {
		licenseID := v.GetString("license-id")
		appSlug := v.GetString("app-slug")

		fmt.Fprintf(os.Stderr, "Auto-uploading bundle to replicated.app...\n")
		if err := supportbundle.UploadBundleAutoDetect(response.ArchivePath, licenseID, appSlug); err != nil {
			fmt.Fprintf(os.Stderr, "Auto-upload failed: %v\n", err)
			fmt.Fprintf(os.Stderr, "You can manually upload the bundle using: support-bundle upload %s\n", response.ArchivePath)
		} else {
			response.FileUploaded = true
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

		fmt.Printf("\nA support bundle was generated and saved at %s. Please send this file to your software vendor for support.\n", response.ArchivePath)
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
func loadSupportBundleSpecsFromURIs(ctx context.Context, kinds *loader.TroubleshootKinds) error {
	moreKinds := loader.NewTroubleshootKinds()

	// iterate through original kinds and replace any support bundle spec with provided uri spec
	for _, s := range kinds.SupportBundlesV1Beta2 {
		if s.Spec.Uri == "" || !util.IsURL(s.Spec.Uri) {
			moreKinds.SupportBundlesV1Beta2 = append(moreKinds.SupportBundlesV1Beta2, s)
			continue
		}

		// We are using LoadSupportBundleSpec function here since it handles prompting
		// users to accept insecure connections
		// There is an opportunity to refactor this code in favour of the Loader APIs
		// TODO: Pass ctx to LoadSupportBundleSpec
		rawSpec, err := supportbundle.LoadSupportBundleSpec(s.Spec.Uri)
		if err != nil {
			// add back original spec
			moreKinds.SupportBundlesV1Beta2 = append(moreKinds.SupportBundlesV1Beta2, s)
			// In the event a spec can't be loaded, we'll just skip it and print a warning
			klog.Warningf("unable to load support bundle from URI: %q: %v", s.Spec.Uri, err)
			continue
		}
		k, err := loader.LoadSpecs(ctx, loader.LoadOptions{RawSpec: string(rawSpec)})
		if err != nil {
			// add back original spec
			moreKinds.SupportBundlesV1Beta2 = append(moreKinds.SupportBundlesV1Beta2, s)
			klog.Warningf("unable to load spec: %v", err)
			continue
		}

		if len(k.SupportBundlesV1Beta2) == 0 {
			// add back original spec
			moreKinds.SupportBundlesV1Beta2 = append(moreKinds.SupportBundlesV1Beta2, s)
			klog.Warningf("no support bundle spec found in URI: %s", s.Spec.Uri)
			continue
		}

		// finally append the uri spec
		moreKinds.SupportBundlesV1Beta2 = append(moreKinds.SupportBundlesV1Beta2, k.SupportBundlesV1Beta2...)

	}

	kinds.SupportBundlesV1Beta2 = moreKinds.SupportBundlesV1Beta2

	return nil
}

func loadSpecs(ctx context.Context, args []string, client kubernetes.Interface) (*troubleshootv1beta2.SupportBundle, *troubleshootv1beta2.Redactor, error) {
	var (
		kinds     = loader.NewTroubleshootKinds()
		vp        = viper.GetViper()
		redactors = vp.GetStringSlice("redactors")
		allArgs   = append(args, redactors...)
		err       error
	)

	kinds, err = specs.LoadFromCLIArgs(ctx, client, allArgs, vp)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to load specs from CLI args")
	}

	// Load additional specs from support bundle URIs
	// only when no-uri flag is not set and no URLs are provided in the args
	if !viper.GetBool("no-uri") {
		err := loadSupportBundleSpecsFromURIs(ctx, kinds)
		if err != nil {
			klog.Warningf("unable to load support bundles from URIs: %v", err)
		}
	}

	// Check if we have any collectors to run in the troubleshoot specs
	// Skip this check if auto-discovery is enabled, as collectors will be added later
	// Note: RemoteCollectors are still actively used in preflights and host preflights
	if len(kinds.CollectorsV1Beta2) == 0 &&
		len(kinds.HostCollectorsV1Beta2) == 0 &&
		len(kinds.SupportBundlesV1Beta2) == 0 &&
		!vp.GetBool("auto") {
		return nil, nil, types.NewExitCodeError(
			constants.EXIT_CODE_CATCH_ALL,
			errors.New("no collectors specified to run. Use --debug and/or -v=2 to see more information"),
		)
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

	// If auto-discovery is enabled and no support bundle specs were loaded,
	// create a minimal default support bundle spec for auto-discovery to work with
	if vp.GetBool("auto") && len(kinds.SupportBundlesV1Beta2) == 0 {
		defaultSupportBundle := troubleshootv1beta2.SupportBundle{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "troubleshoot.replicated.com/v1beta2",
				Kind:       "SupportBundle",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "auto-discovery-default",
			},
			Spec: troubleshootv1beta2.SupportBundleSpec{
				Collectors: []*troubleshootv1beta2.Collect{}, // Empty collectors - will be populated by auto-discovery
			},
		}
		kinds.SupportBundlesV1Beta2 = append(kinds.SupportBundlesV1Beta2, defaultSupportBundle)
		klog.V(2).Info("Created default support bundle spec for auto-discovery")
	}

	var enableRunHostCollectorsInPod bool

	for _, sb := range kinds.SupportBundlesV1Beta2 {
		sb := sb
		mainBundle = supportbundle.ConcatSpec(mainBundle, &sb)
		//check if sb has metadata and if it has RunHostCollectorsInPod set to true
		if !reflect.DeepEqual(sb.ObjectMeta, metav1.ObjectMeta{}) && sb.Spec.RunHostCollectorsInPod {
			enableRunHostCollectorsInPod = sb.Spec.RunHostCollectorsInPod
		}
	}
	mainBundle.Spec.RunHostCollectorsInPod = enableRunHostCollectorsInPod

	for _, c := range kinds.CollectorsV1Beta2 {
		mainBundle.Spec.Collectors = util.Append(mainBundle.Spec.Collectors, c.Spec.Collectors)
	}

	for _, hc := range kinds.HostCollectorsV1Beta2 {
		mainBundle.Spec.HostCollectors = util.Append(mainBundle.Spec.HostCollectors, hc.Spec.Collectors)
	}

	// Don't add default collectors if auto-discovery is enabled, as auto-discovery will add them
	if !(len(mainBundle.Spec.HostCollectors) > 0 && len(mainBundle.Spec.Collectors) == 0) && !vp.GetBool("auto") {
		// Always add default collectors unless we only have host collectors or auto-discovery is enabled
		// We need to add them here so when we --dry-run, these collectors
		// are included. supportbundle.runCollectors duplicates this bit.
		// We'll need to refactor it out later when its clearer what other
		// code depends on this logic e.g KOTS
		mainBundle.Spec.Collectors = collect.EnsureCollectorInList(
			mainBundle.Spec.Collectors,
			troubleshootv1beta2.Collect{ClusterInfo: &troubleshootv1beta2.ClusterInfo{}},
		)
		mainBundle.Spec.Collectors = collect.EnsureCollectorInList(
			mainBundle.Spec.Collectors,
			troubleshootv1beta2.Collect{ClusterResources: &troubleshootv1beta2.ClusterResources{}},
		)
	}

	additionalRedactors := &troubleshootv1beta2.Redactor{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "troubleshoot.replicated.com/v1beta2",
			Kind:       "Redactor",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "merged-redactors-spec",
		},
	}
	for _, r := range kinds.RedactorsV1Beta2 {
		additionalRedactors.Spec.Redactors = util.Append(additionalRedactors.Spec.Redactors, r.Spec.Redactors)
	}

	// dedupe specs
	mainBundle.Spec.Collectors = util.Dedup(mainBundle.Spec.Collectors)
	mainBundle.Spec.Analyzers = util.Dedup(mainBundle.Spec.Analyzers)
	mainBundle.Spec.HostCollectors = util.Dedup(mainBundle.Spec.HostCollectors)
	mainBundle.Spec.HostAnalyzers = util.Dedup(mainBundle.Spec.HostAnalyzers)

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
		return "", fmt.Errorf("\r * Failed to format analysis: %v", err)
	}
	return string(formatted), nil
}

// ValidateTokenizationFlags validates tokenization flag combinations
func ValidateTokenizationFlags(v *viper.Viper) error {
	// Verify tokenization mode early (before collection starts)
	if v.GetBool("verify-tokenization") {
		if err := VerifyTokenizationSetup(v); err != nil {
			return errors.Wrap(err, "tokenization verification failed")
		}
		fmt.Println("✅ Tokenization verification passed")
		os.Exit(0) // Exit after verification
	}

	// Encryption requires redaction map
	if v.GetBool("encrypt-redaction-map") && v.GetString("redaction-map") == "" {
		return errors.New("--encrypt-redaction-map requires --redaction-map to be specified")
	}

	// Redaction map requires tokenization or redaction to be enabled
	if v.GetString("redaction-map") != "" {
		if !v.GetBool("tokenize") && !v.GetBool("redact") {
			return errors.New("--redaction-map requires either --tokenize or --redact to be enabled")
		}
	}

	// Custom token prefix requires tokenization
	if v.GetString("token-prefix") != "" && !v.GetBool("tokenize") {
		return errors.New("--token-prefix requires --tokenize to be enabled")
	}

	// Bundle ID requires tokenization
	if v.GetString("bundle-id") != "" && !v.GetBool("tokenize") {
		return errors.New("--bundle-id requires --tokenize to be enabled")
	}

	// Tokenization stats requires tokenization
	if v.GetBool("tokenization-stats") && !v.GetBool("tokenize") {
		return errors.New("--tokenization-stats requires --tokenize to be enabled")
	}

	return nil
}

// VerifyTokenizationSetup verifies tokenization configuration without collecting data
func VerifyTokenizationSetup(v *viper.Viper) error {
	fmt.Println("🔍 Verifying tokenization setup...")

	// Test 1: Environment variable check
	if v.GetBool("tokenize") {
		os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
		defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	}

	// Test 2: Tokenizer initialization
	redact.ResetGlobalTokenizer()
	tokenizer := redact.GetGlobalTokenizer()

	if v.GetBool("tokenize") && !tokenizer.IsEnabled() {
		return errors.New("tokenizer is not enabled despite --tokenize flag")
	}

	if !v.GetBool("tokenize") && tokenizer.IsEnabled() {
		return errors.New("tokenizer is enabled despite --tokenize flag being false")
	}

	fmt.Printf("  ✅ Tokenizer state: %v\n", tokenizer.IsEnabled())

	// Test 3: Token generation
	if tokenizer.IsEnabled() {
		testToken := tokenizer.TokenizeValue("test-secret", "verification")
		if !tokenizer.ValidateToken(testToken) {
			return errors.Errorf("generated test token is invalid: %s", testToken)
		}
		fmt.Printf("  ✅ Test token generated: %s\n", testToken)
	}

	// Test 4: Custom token prefix validation
	if customPrefix := v.GetString("token-prefix"); customPrefix != "" {
		if !strings.Contains(customPrefix, "%s") {
			return errors.Errorf("custom token prefix must contain %%s placeholders: %s", customPrefix)
		}
		fmt.Printf("  ✅ Custom token prefix validated: %s\n", customPrefix)
	}

	// Test 5: Redaction map path validation
	if mapPath := v.GetString("redaction-map"); mapPath != "" {
		// Check if directory exists
		dir := filepath.Dir(mapPath)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return errors.Errorf("redaction map directory does not exist: %s", dir)
		}
		fmt.Printf("  ✅ Redaction map path validated: %s\n", mapPath)

		// Test file creation (and cleanup)
		testFile := mapPath + ".test"
		if err := os.WriteFile(testFile, []byte("test"), 0600); err != nil {
			return errors.Errorf("cannot create redaction map file: %v", err)
		}
		os.Remove(testFile)
		fmt.Printf("  ✅ File creation permissions verified\n")
	}

	return nil
}
