package collect

import (
	"context"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"golang.org/x/sync/errgroup"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/rest"
)

type HostCollector interface {
	Title() string
	IsExcluded() (bool, error)
	Collect(progressChan chan<- interface{}) (map[string][]byte, error)
	RemoteCollect(progressChan chan<- interface{}) (map[string][]byte, error) // RemoteCollect is used to priviledge pods to collect data from different nodes
}

func GetHostCollector(collector *troubleshootv1beta2.HostCollect, bundlePath string, restConfig *rest.Config) (HostCollector, bool) {
	switch {
	case collector.CPU != nil:
		return &CollectHostCPU{collector.CPU, bundlePath}, true
	case collector.Memory != nil:
		return &CollectHostMemory{collector.Memory, bundlePath}, true
	case collector.TCPLoadBalancer != nil:
		return &CollectHostTCPLoadBalancer{collector.TCPLoadBalancer, bundlePath}, true
	case collector.HTTPLoadBalancer != nil:
		return &CollectHostHTTPLoadBalancer{collector.HTTPLoadBalancer, bundlePath}, true
	case collector.DiskUsage != nil:
		return &CollectHostDiskUsage{collector.DiskUsage, bundlePath}, true
	case collector.TCPPortStatus != nil:
		return &CollectHostTCPPortStatus{collector.TCPPortStatus, bundlePath}, true
	case collector.UDPPortStatus != nil:
		return &CollectHostUDPPortStatus{collector.UDPPortStatus, bundlePath}, true
	case collector.HTTP != nil:
		return &CollectHostHTTP{collector.HTTP, bundlePath}, true
	case collector.Time != nil:
		return &CollectHostTime{collector.Time, bundlePath}, true
	case collector.BlockDevices != nil:
		return &CollectHostBlockDevices{collector.BlockDevices, bundlePath}, true
	case collector.SystemPackages != nil:
		return &CollectHostSystemPackages{collector.SystemPackages, bundlePath}, true
	case collector.KernelModules != nil:
		return &CollectHostKernelModules{
			hostCollector: collector.KernelModules,
			BundlePath:    bundlePath,
			loadable:      kernelModulesLoadable{},
			loaded:        kernelModulesLoaded{},
		}, true
	case collector.TCPConnect != nil:
		return &CollectHostTCPConnect{collector.TCPConnect, bundlePath}, true
	case collector.IPV4Interfaces != nil:
		return &CollectHostIPV4Interfaces{collector.IPV4Interfaces, bundlePath}, true
	case collector.SubnetAvailable != nil:
		return &CollectHostSubnetAvailable{collector.SubnetAvailable, bundlePath}, true
	case collector.FilesystemPerformance != nil:
		return &CollectHostFilesystemPerformance{collector.FilesystemPerformance, bundlePath}, true
	case collector.Certificate != nil:
		return &CollectHostCertificate{collector.Certificate, bundlePath}, true
	case collector.CertificatesCollection != nil:
		return &CollectHostCertificatesCollection{collector.CertificatesCollection, bundlePath}, true
	case collector.HostServices != nil:
		return &CollectHostServices{collector.HostServices, bundlePath}, true
	case collector.HostOS != nil:
		c := &CollectHostOS{collector.HostOS, bundlePath, restConfig, "replicated/troubleshoot:latest", "", "", "default", (120 * time.Second), "hostos-remote", nil}
		return c, true
	case collector.HostRun != nil:
		return &CollectHostRun{collector.HostRun, bundlePath}, true
	case collector.HostCopy != nil:
		return &CollectHostCopy{collector.HostCopy, bundlePath}, true
	case collector.HostKernelConfigs != nil:
		return &CollectHostKernelConfigs{collector.HostKernelConfigs, bundlePath}, true
	case collector.HostJournald != nil:
		return &CollectHostJournald{collector.HostJournald, bundlePath}, true
	case collector.HostCGroups != nil:
		return &CollectHostCGroups{collector.HostCGroups, bundlePath}, true
	case collector.HostDNS != nil:
		return &CollectHostDNS{collector.HostDNS, bundlePath}, true
	default:
		return nil, false
	}
}

func hostCollectorTitleOrDefault(meta troubleshootv1beta2.HostCollectorMeta, defaultTitle string) string {
	if meta.CollectorName != "" {
		return meta.CollectorName
	}
	return defaultTitle
}

func runRemote(ctx context.Context, runner runner, nodes []string, collector *troubleshootv1beta2.HostCollect, nameGenerator names.NameGenerator, namePrefix string, namespace string) (map[string][]byte, error) {
	g, ctx := errgroup.WithContext(ctx)
	results := make(chan map[string][]byte, len(nodes))

	for _, node := range nodes {
		node := node
		g.Go(func() error {
			// May need to evaluate error and log warning.  Otherwise any error
			// here will cancel the context of other goroutines and no results
			// will be returned.
			return runner.run(ctx, collector, namespace, nameGenerator.GenerateName(namePrefix+"-"), node, results)
		})
	}

	// Wait for all collectors to complete or return the first error.
	if err := g.Wait(); err != nil {
		return nil, errors.Wrap(err, "failed remote collection")
	}
	close(results)

	output := make(map[string][]byte)
	for result := range results {
		r := result
		for k, v := range r {
			output[k] = v
		}
	}

	return output, nil
}
