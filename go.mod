module github.com/replicatedhq/troubleshoot

go 1.12

require (
	github.com/ahmetalpbalkan/go-cursor v0.0.0-20131010032410-8136607ea412
	github.com/blang/semver v3.5.1+incompatible
	github.com/fatih/color v1.7.0
	github.com/gin-gonic/gin v1.4.0
	github.com/gizak/termui/v3 v3.1.0
	github.com/hashicorp/go-getter v1.3.1-0.20190627223108-da0323b9545e
	github.com/hashicorp/go-multierror v1.0.0
	github.com/manifoldco/promptui v0.3.2
	github.com/mattn/go-isatty v0.0.9
	github.com/mholt/archiver v3.1.1+incompatible
	github.com/nwaples/rardecode v1.1.0 // indirect
	github.com/pierrec/lz4 v2.4.1+incompatible // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/procfs v0.0.5 // indirect
	github.com/replicatedhq/kots v1.13.3
	github.com/spf13/cobra v0.0.5
	github.com/spf13/viper v1.4.0
	github.com/stretchr/testify v1.4.0
	github.com/tj/go-spin v1.1.0
	go.undefinedlabs.com/scopeagent v0.1.7
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.17.2
	k8s.io/apiextensions-apiserver v0.17.2
	k8s.io/apimachinery v0.17.2
	k8s.io/cli-runtime v0.17.0
	k8s.io/client-go v0.17.2
	sigs.k8s.io/controller-runtime v0.4.0
)

replace github.com/appscode/jsonpatch => github.com/gomodules/jsonpatch v2.0.1+incompatible

replace github.com/nicksnyder/go-i18n => github.com/nicksnyder/go-i18n v1.10.1
