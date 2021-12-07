module github.com/replicatedhq/troubleshoot

go 1.17

require (
	github.com/ahmetalpbalkan/go-cursor v0.0.0-20131010032410-8136607ea412
	github.com/blang/semver v3.5.1+incompatible
	github.com/containers/image/v5 v5.17.0
	github.com/docker/distribution v2.7.1+incompatible
	github.com/fatih/color v1.13.0
	github.com/go-redis/redis/v7 v7.4.1
	github.com/go-sql-driver/mysql v1.6.0
	github.com/gobwas/glob v0.2.3
	github.com/godbus/dbus v4.1.0+incompatible
	github.com/google/gofuzz v1.2.0
	github.com/gorilla/handlers v1.5.1
	github.com/hashicorp/go-getter v1.5.9
	github.com/hashicorp/go-multierror v1.1.1
	github.com/lib/pq v1.10.4
	github.com/longhorn/go-iscsi-helper v0.0.0-20210330030558-49a327fb024e
	github.com/manifoldco/promptui v0.9.0
	github.com/mattn/go-isatty v0.0.14
	github.com/mholt/archiver v3.1.1+incompatible
	github.com/pkg/errors v0.9.1
	github.com/replicatedhq/termui/v3 v3.1.1-0.20200811145416-f40076d26851
	github.com/satori/go.uuid v1.2.0
	github.com/segmentio/ksuid v1.0.4
	github.com/shirou/gopsutil v3.21.11+incompatible
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.9.0
	github.com/stretchr/testify v1.7.0
	github.com/tj/go-spin v1.1.0
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.22.4
	k8s.io/apiextensions-apiserver v0.22.4
	k8s.io/apimachinery v0.22.4
	k8s.io/apiserver v0.22.4
	k8s.io/cli-runtime v0.22.4
	k8s.io/client-go v0.22.4
	sigs.k8s.io/controller-runtime v0.10.3
)

require (
	github.com/chzyer/logex v1.1.11-0.20160617073814-96a4d311aa9b // indirect
	github.com/dsnet/compress v0.0.1 // indirect
	github.com/elazarl/goproxy v0.0.0-20191011121108-aa519ddbe484 // indirect
	github.com/frankban/quicktest v1.14.0 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/hashicorp/go-version v1.3.0 // indirect
	github.com/nwaples/rardecode v1.1.2 // indirect
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/tklauser/go-sysconf v0.3.9 // indirect
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	periph.io/x/periph v3.6.8+incompatible
)

replace (
	github.com/StackExchange/wmi => github.com/yusufpapurcu/wmi v1.2.2
	github.com/go-ole/go-ole => github.com/go-ole/go-ole v1.2.6 // needed for arm builds
)
