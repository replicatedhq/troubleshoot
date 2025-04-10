package collect

import (
	"context"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"golang.org/x/sync/errgroup"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	remoteCollectorDefaultInterval = 1 * time.Second
	remoteCollectorNamePrefix      = "preflight-remote"
)

type RemoteCollector struct {
	Collect       *troubleshootv1beta2.RemoteCollect
	Redact        bool
	RBACErrors    []error
	ClientConfig  *rest.Config
	Image         string
	PullPolicy    string
	LabelSelector string
	Namespace     string
	BundlePath    string
	Timeout       time.Duration
	NamePrefix    string
}

type RemoteCollectors []*RemoteCollector

type runner interface {
	run(ctx context.Context, collector *troubleshootv1beta2.HostCollect, namespace string, name string, nodeName string, results chan<- map[string][]byte) error
}

// checks if a given collector has a spec with 'exclude' that evaluates to true.
func (c *RemoteCollector) IsExcluded() bool {
	if c.Collect.KernelModules != nil {
		isExcludedResult, err := isExcluded(c.Collect.KernelModules.Exclude)
		if err != nil {
			return true
		}
		if isExcludedResult {
			return true
		}
	}
	return false
}

func (c *RemoteCollector) RunCollectorSync(globalRedactors []*troubleshootv1beta2.Redact) (CollectorResult, error) {
	var err error
	defer func() {
		if r := recover(); r != nil {
			err = errors.Errorf("recovered from panic: %v", r)
		}
	}()

	if c.IsExcluded() {
		return nil, nil
	}

	hostCollector, err := c.toHostCollector()
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert to host collector")
	}

	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, err
	}

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, errors.Wrap(err, "failed to add runtime scheme")
	}

	runner := &podRunner{
		client:       client,
		scheme:       scheme,
		image:        c.Image,
		pullPolicy:   c.PullPolicy,
		waitInterval: remoteCollectorDefaultInterval,
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	// Get all the nodes where we should run.
	nodes, err := listNodesNamesInSelector(ctx, client, c.LabelSelector)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the list of nodes matching a nodeSelector")
	}

	if c.NamePrefix == "" {
		c.NamePrefix = remoteCollectorNamePrefix
	}

	result, err := c.RunRemote(ctx, runner, nodes, hostCollector, names.SimpleNameGenerator, c.NamePrefix)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run collector remotely")
	}

	if !c.Redact {
		return result, nil
	}

	if err = RedactResult("", result, globalRedactors, nil); err != nil {
		// Returning result on error to be consistent with local collector.
		return result, errors.Wrap(err, "failed to redact remote collector results")
	}
	return result, nil
}

func (c *RemoteCollector) RunRemote(ctx context.Context, runner runner, nodes []string, collector *troubleshootv1beta2.HostCollect, nameGenerator names.NameGenerator, namePrefix string) (map[string][]byte, error) {
	g, ctx := errgroup.WithContext(ctx)
	results := make(chan map[string][]byte, len(nodes))

	for _, node := range nodes {
		node := node
		g.Go(func() error {
			// May need to evaluate error and log warning.  Otherwise any error
			// here will cancel the context of other goroutines and no results
			// will be returned.
			return runner.run(ctx, collector, c.Namespace, nameGenerator.GenerateName(namePrefix+"-"), node, results)
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

func (c *RemoteCollector) GetDisplayName() string {
	return c.Collect.GetName()
}

// toHostCollector converts the remote collector to a local collector.
func (c *RemoteCollector) toHostCollector() (*troubleshootv1beta2.HostCollect, error) {
	hostCollect := &troubleshootv1beta2.HostCollect{}

	switch {
	case c.Collect.CPU != nil:
		hostCollect.CPU = &troubleshootv1beta2.CPU{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.CPU.CollectorName,
				Exclude:       c.Collect.CPU.Exclude,
			},
		}
	case c.Collect.Memory != nil:
		hostCollect.Memory = &troubleshootv1beta2.Memory{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.Memory.CollectorName,
				Exclude:       c.Collect.Memory.Exclude,
			},
		}
	case c.Collect.TCPLoadBalancer != nil:
		hostCollect.TCPLoadBalancer = &troubleshootv1beta2.TCPLoadBalancer{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.TCPLoadBalancer.CollectorName,
				Exclude:       c.Collect.TCPLoadBalancer.Exclude,
			},
			Address: c.Collect.TCPLoadBalancer.Address,
			Port:    c.Collect.TCPLoadBalancer.Port,
			Timeout: c.Collect.TCPLoadBalancer.Timeout,
		}
	case c.Collect.HTTPLoadBalancer != nil:
		hostCollect.HTTPLoadBalancer = &troubleshootv1beta2.HTTPLoadBalancer{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.HTTPLoadBalancer.CollectorName,
				Exclude:       c.Collect.HTTPLoadBalancer.Exclude,
			},
			Address: c.Collect.TCPLoadBalancer.Address,
			Port:    c.Collect.TCPLoadBalancer.Port,
			Timeout: c.Collect.TCPLoadBalancer.Timeout,
		}
	case c.Collect.DiskUsage != nil:
		hostCollect.DiskUsage = &troubleshootv1beta2.DiskUsage{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.DiskUsage.CollectorName,
				Exclude:       c.Collect.DiskUsage.Exclude,
			},
			Path: c.Collect.DiskUsage.Path,
		}
	case c.Collect.TCPPortStatus != nil:
		hostCollect.TCPPortStatus = &troubleshootv1beta2.TCPPortStatus{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.TCPPortStatus.CollectorName,
				Exclude:       c.Collect.TCPPortStatus.Exclude,
			},
			Interface: c.Collect.TCPPortStatus.Interface,
			Port:      c.Collect.TCPPortStatus.Port,
		}
	case c.Collect.UDPPortStatus != nil:
		hostCollect.UDPPortStatus = &troubleshootv1beta2.UDPPortStatus{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.UDPPortStatus.CollectorName,
				Exclude:       c.Collect.UDPPortStatus.Exclude,
			},
			Interface: c.Collect.UDPPortStatus.Interface,
			Port:      c.Collect.UDPPortStatus.Port,
		}
	case c.Collect.HTTP != nil:
		hostCollect.HTTP = &troubleshootv1beta2.HostHTTP{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.HTTP.CollectorName,
				Exclude:       c.Collect.HTTP.Exclude,
			},
			Get:  c.Collect.HTTP.Get,
			Post: c.Collect.HTTP.Post,
			Put:  c.Collect.HTTP.Put,
		}
	case c.Collect.Time != nil:
		hostCollect.Time = &troubleshootv1beta2.HostTime{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.Time.CollectorName,
				Exclude:       c.Collect.Time.Exclude,
			},
		}
	case c.Collect.BlockDevices != nil:
		hostCollect.BlockDevices = &troubleshootv1beta2.HostBlockDevices{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.BlockDevices.CollectorName,
				Exclude:       c.Collect.BlockDevices.Exclude,
			},
		}
	case c.Collect.SystemPackages != nil:
		hostCollect.SystemPackages = &troubleshootv1beta2.HostSystemPackages{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.SystemPackages.CollectorName,
				Exclude:       c.Collect.SystemPackages.Exclude,
			},
		}
	case c.Collect.KernelModules != nil:
		hostCollect.KernelModules = &troubleshootv1beta2.HostKernelModules{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.KernelModules.CollectorName,
				Exclude:       c.Collect.KernelModules.Exclude,
			},
		}
	case c.Collect.TCPConnect != nil:
		hostCollect.TCPConnect = &troubleshootv1beta2.TCPConnect{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.TCPConnect.CollectorName,
				Exclude:       c.Collect.TCPConnect.Exclude,
			},
			Address: c.Collect.TCPConnect.Address,
			Timeout: c.Collect.TCPConnect.Timeout,
		}
	case c.Collect.IPV4Interfaces != nil:
		hostCollect.IPV4Interfaces = &troubleshootv1beta2.IPV4Interfaces{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.IPV4Interfaces.CollectorName,
				Exclude:       c.Collect.IPV4Interfaces.Exclude,
			},
		}
	case c.Collect.SubnetAvailable != nil:
		hostCollect.SubnetAvailable = &troubleshootv1beta2.SubnetAvailable{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.IPV4Interfaces.CollectorName,
				Exclude:       c.Collect.IPV4Interfaces.Exclude,
			},
			CIDRRangeAlloc: c.Collect.SubnetAvailable.CIDRRangeAlloc,
			DesiredCIDR:    c.Collect.SubnetAvailable.DesiredCIDR,
		}
	case c.Collect.FilesystemPerformance != nil:
		hostCollect.FilesystemPerformance = &troubleshootv1beta2.FilesystemPerformance{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.FilesystemPerformance.CollectorName,
				Exclude:       c.Collect.FilesystemPerformance.Exclude,
			},
			OperationSizeBytes:          c.Collect.FilesystemPerformance.OperationSizeBytes,
			Directory:                   c.Collect.FilesystemPerformance.Directory,
			FileSize:                    c.Collect.FilesystemPerformance.FileSize,
			Sync:                        c.Collect.FilesystemPerformance.Sync,
			Datasync:                    c.Collect.FilesystemPerformance.Datasync,
			Timeout:                     c.Collect.FilesystemPerformance.Timeout,
			EnableBackgroundIOPS:        c.Collect.FilesystemPerformance.EnableBackgroundIOPS,
			BackgroundIOPSWarmupSeconds: c.Collect.FilesystemPerformance.BackgroundIOPSWarmupSeconds,
			BackgroundWriteIOPS:         c.Collect.FilesystemPerformance.BackgroundWriteIOPS,
			BackgroundReadIOPS:          c.Collect.FilesystemPerformance.BackgroundReadIOPS,
			BackgroundWriteIOPSJobs:     c.Collect.FilesystemPerformance.BackgroundWriteIOPSJobs,
			BackgroundReadIOPSJobs:      c.Collect.FilesystemPerformance.BackgroundReadIOPSJobs,
		}
	case c.Collect.Certificate != nil:
		hostCollect.Certificate = &troubleshootv1beta2.Certificate{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.Certificate.CollectorName,
				Exclude:       c.Collect.Certificate.Exclude,
			},
			CertificatePath: c.Collect.Certificate.CertificatePath,
			KeyPath:         c.Collect.Certificate.KeyPath,
		}
	case c.Collect.HostServices != nil:
		hostCollect.HostServices = &troubleshootv1beta2.HostServices{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.HostServices.CollectorName,
				Exclude:       c.Collect.HostServices.Exclude,
			},
		}
	case c.Collect.HostOS != nil:
		hostCollect.HostOS = &troubleshootv1beta2.HostOS{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.HostOS.CollectorName,
				Exclude:       c.Collect.HostOS.Exclude,
			},
		}
	default:
		return nil, errors.New("no spec found to run")
	}
	return hostCollect, nil
}

func (c *RemoteCollector) CheckRBAC(ctx context.Context) error {
	if c.IsExcluded() {
		return nil // excluded collectors require no permissions
	}

	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return errors.Wrap(err, "failed to create client from config")
	}

	forbidden := make([]error, 0)

	specs := c.Collect.AccessReviewSpecs(c.Namespace)
	for _, spec := range specs {
		sar := &authorizationv1.SelfSubjectAccessReview{
			Spec: spec,
		}

		resp, err := client.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
		if err != nil {
			return errors.Wrap(err, "failed to run subject review")
		}

		if !resp.Status.Allowed { // all other fields of Status are empty...
			forbidden = append(forbidden, RBACError{
				DisplayName: c.GetDisplayName(),
				Namespace:   spec.ResourceAttributes.Namespace,
				Resource:    spec.ResourceAttributes.Resource,
				Verb:        spec.ResourceAttributes.Verb,
			})
		}
	}
	c.RBACErrors = forbidden

	return nil
}

func (cs RemoteCollectors) CheckRBAC(ctx context.Context) error {
	for _, c := range cs {
		if err := c.CheckRBAC(ctx); err != nil {
			return errors.Wrap(err, "failed to check RBAC")
		}
	}
	return nil
}
