package collect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type HostCollector interface {
	Title() string
	IsExcluded() (bool, error)
	SkipRedaction() bool
	Collect(progressChan chan<- interface{}) (map[string][]byte, error)
}

type RemoteCollectParams struct {
	ProgressChan  chan<- interface{}
	HostCollector *troubleshootv1beta2.HostCollect
	BundlePath    string
	ClientConfig  *rest.Config // specify actual type
	Image         string
	PullPolicy    string        // specify actual type if needed
	Timeout       time.Duration // specify duration type if needed
	LabelSelector string
	NamePrefix    string
	Namespace     string
	Title         string
}

func GetHostCollector(collector *troubleshootv1beta2.HostCollect, bundlePath string) (HostCollector, bool) {
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
			loaded: kernelModulesLoaded{
				fs: os.DirFS("/"),
			},
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
		return &CollectHostOS{collector.HostOS, bundlePath}, true
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
	case collector.NetworkNamespaceConnectivity != nil:
		return &CollectHostNetworkNamespaceConnectivity{collector.NetworkNamespaceConnectivity, bundlePath}, true
	case collector.HostSysctl != nil:
		return &CollectHostSysctl{collector.HostSysctl, bundlePath}, true
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

func RemoteHostCollect(ctx context.Context, params RemoteCollectParams) (map[string][]byte, error) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, errors.Wrap(err, "failed to add runtime scheme")
	}

	client, err := kubernetes.NewForConfig(params.ClientConfig)
	if err != nil {
		return nil, err
	}

	runner := &podRunner{
		client:       client,
		scheme:       scheme,
		image:        params.Image,
		pullPolicy:   params.PullPolicy,
		waitInterval: remoteCollectorDefaultInterval,
	}

	// Get all the nodes where we should run.
	nodes, err := listNodesNamesInSelector(ctx, client, params.LabelSelector)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the list of nodes matching a nodeSelector")
	}

	if params.NamePrefix == "" {
		params.NamePrefix = remoteCollectorNamePrefix
	}

	result, err := runRemote(ctx, runner, nodes, params.HostCollector, names.SimpleNameGenerator, params.NamePrefix, params.Namespace)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run collector remotely")
	}

	allCollectedData := mapCollectorResultToOutput(result, params)
	output := NewResult()

	// save the first result we find in the node and save it
	for node, result := range allCollectedData {
		var nodeResult map[string]string
		if err := json.Unmarshal(result, &nodeResult); err != nil {
			return nil, errors.Wrap(err, "failed to marshal node results")
		}

		for file, collectorResult := range nodeResult {
			directory := filepath.Dir(file)
			fileName := filepath.Base(file)
			// expected file name for remote collectors will be the normal path separated by / and the node name
			output.SaveResult(params.BundlePath, fmt.Sprintf("%s/%s/%s", directory, node, fileName), bytes.NewBufferString(collectorResult))
		}
	}

	// check if NODE_LIST_FILE exists
	_, err = os.Stat(constants.NODE_LIST_FILE)
	// if it not exists, save the nodes list
	if err != nil {
		nodesBytes, err := json.MarshalIndent(HostOSInfoNodes{Nodes: nodes}, "", " ")
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal host os info nodes")
		}
		output.SaveResult(params.BundlePath, constants.NODE_LIST_FILE, bytes.NewBuffer(nodesBytes))
	}
	return output, nil
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

func mapCollectorResultToOutput(result map[string][]byte, params RemoteCollectParams) map[string][]byte {
	allCollectedData := make(map[string][]byte)

	for k, v := range result {
		if curBytes, ok := allCollectedData[k]; ok {
			var curResults map[string]string
			if err := json.Unmarshal(curBytes, &curResults); err != nil {
				params.ProgressChan <- errors.Errorf("failed to read existing results for collector %s: %v\n", params.Title, err)
				continue
			}
			var newResults map[string]string
			if err := json.Unmarshal(v, &newResults); err != nil {
				params.ProgressChan <- errors.Errorf("failed to read new results for collector %s: %v\n", params.Title, err)
				continue
			}
			for file, data := range newResults {
				curResults[file] = data
			}
			combinedResults, err := json.Marshal(curResults)
			if err != nil {
				params.ProgressChan <- errors.Errorf("failed to combine results for collector %s: %v\n", params.Title, err)
				continue
			}
			allCollectedData[k] = combinedResults
		} else {
			allCollectedData[k] = v
		}

	}
	return allCollectedData
}
