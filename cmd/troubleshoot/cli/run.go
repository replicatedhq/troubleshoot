package cli

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"net/http"
	"os/signal"

	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	cursor "github.com/ahmetalpbalkan/go-cursor"
	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/client/troubleshootclientset/scheme"
	troubleshootclientsetscheme "github.com/replicatedhq/troubleshoot/pkg/client/troubleshootclientset/scheme"
	"github.com/replicatedhq/troubleshoot/pkg/convert"
	"github.com/replicatedhq/troubleshoot/pkg/docrewrite"
	"github.com/replicatedhq/troubleshoot/pkg/httputil"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/replicatedhq/troubleshoot/pkg/supportbundle"
	"github.com/spf13/viper"
	spin "github.com/tj/go-spin"
)

func runTroubleshoot(v *viper.Viper, arg string) error {
	fmt.Print(cursor.Hide())
	defer fmt.Print(cursor.Show())

	go func() {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt)
		<-signalChan
		fmt.Print(cursor.Show())
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

	collectorContent, err := supportbundle.LoadSupportBundleSpec(arg)
	if err != nil {
		return errors.Wrap(err, "failed to load collector spec")
	}

	multidocs := strings.Split(string(collectorContent), "\n---\n")

	// we support both raw collector kinds and supportbundle kinds here
	supportBundle, err := supportbundle.ParseSupportBundleFromDoc([]byte(multidocs[0]))
	if err != nil {
		return errors.Wrap(err, "failed to parse collector")
	}

	troubleshootclientsetscheme.AddToScheme(scheme.Scheme)
	decode := scheme.Codecs.UniversalDeserializer().Decode

	additionalRedactors := &troubleshootv1beta2.Redactor{}
	for idx, redactor := range v.GetStringSlice("redactors") {
		redactorObj, err := supportbundle.GetRedactorFromURI(redactor)
		if err != nil {
			return errors.Wrapf(err, "failed to get redactor spec %s, #%d", redactor, idx)
		}

		if redactorObj != nil {
			additionalRedactors.Spec.Redactors = append(additionalRedactors.Spec.Redactors, redactorObj.Spec.Redactors...)
		}
	}

	for i, additionalDoc := range multidocs {
		if i == 0 {
			continue
		}
		additionalDoc, err := docrewrite.ConvertToV1Beta2([]byte(additionalDoc))
		if err != nil {
			return errors.Wrap(err, "failed to convert to v1beta2")
		}
		obj, _, err := decode(additionalDoc, nil, nil)
		if err != nil {
			return errors.Wrapf(err, "failed to parse additional doc %d", i)
		}
		multidocRedactors, ok := obj.(*troubleshootv1beta2.Redactor)
		if !ok {
			continue
		}
		additionalRedactors.Spec.Redactors = append(additionalRedactors.Spec.Redactors, multidocRedactors.Spec.Redactors...)
	}

	s := spin.New()
	interactive := v.GetBool("interactive") && isatty.IsTerminal(os.Stdout.Fd())
	finishedCh := make(chan bool, 1)
	progressChan := make(chan interface{}) // non-zero buffer can result in missed messages
	isFinishedChClosed := false
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
					if interactive {
						fmt.Printf("\r%s \033[36mCollecting support bundle\033[m %s", cursor.ClearEntireLine(), s.Next())
					}
				} else {
					if interactive {
						fmt.Printf("\r%s \033[36mCollecting support bundle\033[m %s %s", cursor.ClearEntireLine(), s.Next(), currentDir)
					}
				}
			}
		}
	}()
	defer func() {
		if !isFinishedChClosed {
			close(finishedCh)
		}
	}()

	collectorCB := func(c chan interface{}, msg string) {
		c <- fmt.Sprintf("%s", msg)
	}

	createOpts := supportbundle.SupportBundleCreateOpts{
		CollectorProgressCallback: collectorCB,
		CollectWithoutPermissions: v.GetBool("collect-without-permissions"),
		KubernetesRestConfig:      restConfig,
		Namespace:                 v.GetString("namespace"),
		ProgressChan:              progressChan,
		SinceTime:                 sinceTime,
		FromCLI:                   true,
	}

	nonInteractiveOutput := analysisOutput{}

	c := color.New()
	c.Println(fmt.Sprintf("\r%s\r", cursor.ClearEntireLine()))

	response, err := supportbundle.CollectSupportBundleFromSpec(&supportBundle.Spec, additionalRedactors, createOpts)
	if err != nil {
		return errors.Wrap(err, "failed to run collect and analyze process")
	}
	if len(response.AnalyzerResults) > 0 {
		interactive := v.GetBool("interactive") && isatty.IsTerminal(os.Stdout.Fd())

		if interactive {
			close(finishedCh) // this removes the spinner
			isFinishedChClosed = true

			if err := showInteractiveResults(supportBundle.Name, response.AnalyzerResults); err != nil {
				interactive = false
			}
		} else {
			nonInteractiveOutput.Analysis = response.AnalyzerResults
		}
	}

	if !response.FileUploaded {
		if appName := supportBundle.Labels["applicationName"]; appName != "" {
			f := `A support bundle for %s has been created in this directory
named %s. Please upload it on the Troubleshoot page of
the %s Admin Console to begin analysis.`
			fmt.Printf(f, appName, response.ArchivePath, appName)
			return nil
		}

		nonInteractiveOutput.ArchivePath = response.ArchivePath
		output, err := nonInteractiveOutput.FormattedAnalysisOutput()
		if err != nil {
			return errors.Wrap(err, "failed to format non-interactive output")
		}
		fmt.Println(output)
		return nil
	}

	fmt.Printf("\r%s\r", cursor.ClearEntireLine())
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

func (a *analysisOutput) FormattedAnalysisOutput() (outputJson []byte, err error) {
	type convertedOutput struct {
		ConvertedAnalysis []*convert.Result
		ArchivePath       string
	}

	converted := convert.FromAnalyzerResult(a.Analysis)

	o := convertedOutput{
		ConvertedAnalysis: converted,
		ArchivePath:       a.ArchivePath,
	}

	formatted, err := json.MarshalIndent(o, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("\r * Failed to format analysis: %v\n", err)
	}
	return formatted, nil
}
