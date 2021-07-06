package cli

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os/signal"
	"time"

	"os"
	"strings"

	cursor "github.com/ahmetalpbalkan/go-cursor"
	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/cmd/util"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/client/troubleshootclientset/scheme"
	troubleshootclientsetscheme "github.com/replicatedhq/troubleshoot/pkg/client/troubleshootclientset/scheme"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/replicatedhq/troubleshoot/pkg/docrewrite"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/replicatedhq/troubleshoot/pkg/specs"
	"github.com/replicatedhq/troubleshoot/pkg/supportbundle"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	defaultTimeout = 30 * time.Second
)

func runCollect(v *viper.Viper, arg string) error {
	go func() {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt)
		<-signalChan
		fmt.Print(cursor.Show())
		os.Exit(0)
	}()

	var collectorContent []byte
	var err error
	if strings.HasPrefix(arg, "secret/") {
		// format secret/namespace-name/secret-name
		pathParts := strings.Split(arg, "/")
		if len(pathParts) != 3 {
			return errors.Errorf("path %s must have 3 components", arg)
		}

		spec, err := specs.LoadFromSecret(pathParts[1], pathParts[2], "collect-spec")
		if err != nil {
			return errors.Wrap(err, "failed to get spec from secret")
		}

		collectorContent = spec
	} else if _, err = os.Stat(arg); err == nil {
		b, err := ioutil.ReadFile(arg)
		if err != nil {
			return err
		}

		collectorContent = b
	} else {
		if !util.IsURL(arg) {
			return fmt.Errorf("%s is not a URL and was not found (err %s)", arg, err)
		}

		req, err := http.NewRequest("GET", arg, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", "Replicated_Collect/v1beta2")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		collectorContent = body
	}

	collectorContent, err = docrewrite.ConvertToV1Beta2(collectorContent)
	if err != nil {
		return errors.Wrap(err, "failed to convert to v1beta2")
	}

	multidocs := strings.Split(string(collectorContent), "\n---\n")

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

	finishedCh := make(chan bool, 1)
	progressCh := make(chan interface{}) // non-zero buffer can result in missed messages
	isFinishedChClosed := false

	// make sure we don't block any senders
	go func() {
		for {
			select {
			case <-progressCh:
				// if being run as a remote collector, output here will break
				// parsing as stdout and stderr are combined.  This means errors
				// won't be reported.
			case <-finishedCh:
				return
			}
		}
	}()
	defer func() {
		if !isFinishedChClosed {
			close(finishedCh)
		}
	}()

	restConfig, err := k8sutil.GetRESTConfig()
	if err != nil {
		return errors.Wrap(err, "failed to convert kube flags to rest config")
	}

	labelSelector, err := labels.Parse(v.GetString("selector"))
	if err != nil {
		return errors.Wrap(err, "unable to parse selector")
	}

	namespace := v.GetString("namespace")
	if namespace == "" {
		namespace = "default"
	}

	timeout := v.GetDuration("request-timeout")
	if timeout == 0 {
		timeout = defaultTimeout
	}

	createOpts := collect.CollectorRunOpts{
		CollectWithoutPermissions: v.GetBool("collect-without-permissions"),
		KubernetesRestConfig:      restConfig,
		Image:                     v.GetString("collector-image"),
		PullPolicy:                v.GetString("collector-pullpolicy"),
		LabelSelector:             labelSelector.String(),
		Namespace:                 namespace,
		Timeout:                   timeout,
		ProgressChan:              progressCh,
	}

	// we only support HostCollector or RemoteCollector kinds.
	hostCollector, err := collect.ParseHostCollectorFromDoc([]byte(multidocs[0]))
	if err == nil {
		results, err := collect.CollectHost(hostCollector, additionalRedactors, createOpts)
		if err != nil {
			return errors.Wrap(err, "failed to collect from host")
		}
		return showHostStdoutResults(v.GetString("format"), hostCollector.Name, results)
	}

	remoteCollector, err := collect.ParseRemoteCollectorFromDoc([]byte(multidocs[0]))
	if err == nil {
		results, err := collect.CollectRemote(remoteCollector, additionalRedactors, createOpts)
		if err != nil {
			return errors.Wrap(err, "failed to collect from remote host(s)")
		}
		return showRemoteStdoutResults(v.GetString("format"), remoteCollector.Name, results)
	}

	return errors.New("failed to parse hostCollector or remoteCollector")
}
