package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/replicatedhq/troubleshoot/pkg/docrewrite"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	defaultTimeout = 30 * time.Second
)

var opt collect.CollectorRunOpts

func collectFromHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Println("got spec:")
	spec_body, err := io.ReadAll(r.Body)
	fmt.Println(string(spec_body))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Println(err)
		return
	}
	doc, err := docrewrite.ConvertToV1Beta2(spec_body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Println(err)
		return
	}

	multidocs := strings.Split(string(doc), "\n---\n")

	hostCollector, err := collect.ParseHostCollectorFromDoc([]byte(multidocs[0]))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Println(err)
		return
	}
	results, err := collect.CollectHost(hostCollector, nil, opt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Println(err)
		return
	}
	res, err := json.Marshal(results.AllCollectedData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Println(err)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(res)

}

func collectServer(v *viper.Viper) error {
	// make sure we don't block any senders
	progressCh := make(chan interface{})
	defer close(progressCh)
	go func() {
		for range progressCh {
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

	opt = collect.CollectorRunOpts{
		CollectWithoutPermissions: v.GetBool("collect-without-permissions"),
		KubernetesRestConfig:      restConfig,
		Image:                     v.GetString("collector-image"),
		PullPolicy:                v.GetString("collector-pullpolicy"),
		LabelSelector:             labelSelector.String(),
		Namespace:                 namespace,
		Timeout:                   timeout,
		ProgressChan:              progressCh,
	}

	http.HandleFunc("/collect", collectFromHTTP)
	fmt.Println("listening on :8888")
	err = http.ListenAndServe(":8888", nil)
	if err != nil {
		return err
	}
	return nil
}
