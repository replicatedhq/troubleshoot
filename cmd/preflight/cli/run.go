package cli

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
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
	"k8s.io/client-go/kubernetes/scheme"
)

func runPreflights(v *viper.Viper, arg string) error {
	fmt.Print(cursor.Hide())
	defer fmt.Print(cursor.Show())

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

	preflightSpec := obj.(*troubleshootv1beta2.Preflight)

	s := spin.New()
	finishedCh := make(chan bool, 1)
	progressChan := make(chan interface{}, 0) // non-zero buffer will result in missed messages
	go func() {
		for {
			select {
			case msg, ok := <-progressChan:
				if !ok {
					continue
				}
				switch msg := msg.(type) {
				case error:
					c := color.New(color.FgHiRed)
					c.Println(fmt.Sprintf("%s\r * %v", cursor.ClearEntireLine(), msg))
				case string:
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
	defer func() {
		close(finishedCh)
	}()

	restConfig, err := k8sutil.GetRESTConfig()
	if err != nil {
		return errors.Wrap(err, "failed to convert kube flags to rest config")
	}

	collectOpts := preflight.CollectOpts{
		Namespace:              v.GetString("namespace"),
		IgnorePermissionErrors: v.GetBool("collect-without-permissions"),
		ProgressChan:           progressChan,
		KubernetesRestConfig:   restConfig,
	}

	collectResults, err := preflight.Collect(collectOpts, preflightSpec)
	if err != nil {
		if !collectResults.IsRBACAllowed {
			if preflightSpec.Spec.UploadResultsTo != "" {
				err := uploadErrors(preflightSpec.Spec.UploadResultsTo, collectResults.Collectors)
				if err != nil {
					progressChan <- err
				}
			}
		}
		return err
	}

	analyzeResults := collectResults.Analyze()
	if preflightSpec.Spec.UploadResultsTo != "" {
		err := uploadResults(preflightSpec.Spec.UploadResultsTo, analyzeResults)
		if err != nil {
			progressChan <- err
		}
	}

	finishedCh <- true

	if v.GetBool("interactive") {
		if len(analyzeResults) == 0 {
			return errors.New("no data has been collected")
		}
		return showInteractiveResults(preflightSpec.Name, analyzeResults)
	}

	return showStdoutResults(v.GetString("format"), preflightSpec.Name, analyzeResults)
}
