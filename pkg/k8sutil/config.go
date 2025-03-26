package k8sutil

import (
	flag "github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/apimachinery/pkg/api/meta"
)

var (
	kubernetesConfigFlags *genericclioptions.ConfigFlags
)

func init() {
	kubernetesConfigFlags = genericclioptions.NewConfigFlags(false)
}

func AddFlags(flags *flag.FlagSet) {
	kubernetesConfigFlags.AddFlags(flags)
}

func GetKubeconfig() clientcmd.ClientConfig {
	return kubernetesConfigFlags.ToRawKubeConfigLoader()
}

func GetRESTConfig() (*rest.Config, error) {
	return kubernetesConfigFlags.ToRESTConfig()
}

func GetRESTMapper() (meta.RESTMapper, error) {
	return kubernetesConfigFlags.ToRESTMapper()
}
