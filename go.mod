module github.com/replicatedhq/troubleshoot

go 1.12

require (
	github.com/StackExchange/wmi v0.0.0-20210224194228-fe8f1750fd46 // indirect
	github.com/ahmetalpbalkan/go-cursor v0.0.0-20131010032410-8136607ea412
	github.com/blang/semver v3.5.1+incompatible
	github.com/chzyer/logex v1.1.11-0.20160617073814-96a4d311aa9b // indirect
	github.com/containers/image/v5 v5.10.4
	github.com/docker/distribution v2.7.1+incompatible
	github.com/elazarl/goproxy v0.0.0-20191011121108-aa519ddbe484 // indirect
	github.com/fatih/color v1.12.0
	github.com/go-ole/go-ole v1.2.5 // indirect
	github.com/go-redis/redis/v7 v7.2.0
	github.com/go-sql-driver/mysql v1.5.0
	github.com/gobwas/glob v0.2.3
	github.com/godbus/dbus v4.1.0+incompatible
	github.com/google/go-cmp v0.5.6 // indirect
	github.com/google/gofuzz v1.1.0
	github.com/gorilla/handlers v1.5.1
	github.com/hashicorp/go-getter v1.3.1-0.20190627223108-da0323b9545e
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/go-version v1.3.0 // indirect
	github.com/lib/pq v1.3.0
	github.com/longhorn/go-iscsi-helper v0.0.0-20210330030558-49a327fb024e
	github.com/manifoldco/promptui v0.8.0
	github.com/mattn/go-isatty v0.0.12
	github.com/onsi/gomega v1.14.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/replicatedhq/termui/v3 v3.1.1-0.20200811145416-f40076d26851
	github.com/satori/go.uuid v1.2.0
	github.com/segmentio/ksuid v1.0.3
	github.com/shirou/gopsutil v3.21.1+incompatible
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.8.1
	github.com/stretchr/testify v1.7.0
	github.com/tj/go-spin v1.1.0
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/tools v0.1.5 // indirect
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.22.2
	k8s.io/apiextensions-apiserver v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/apiserver v0.22.2
	k8s.io/cli-runtime v0.21.5
	k8s.io/client-go v0.22.2
	sigs.k8s.io/controller-runtime v0.8.3
)

replace sigs.k8s.io/controller-runtime => github.com/kubernetes-sigs/controller-runtime v0.8.3
