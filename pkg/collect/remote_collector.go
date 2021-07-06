package collect

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
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
}

type RemoteCollectors []*RemoteCollector

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

	localCollector := &troubleshootv1beta2.HostCollect{}

	if c.Collect.CPU != nil {
		localCollector.CPU = &troubleshootv1beta2.CPU{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.CPU.CollectorName,
				Exclude:       c.Collect.CPU.Exclude,
			},
		}
	} else if c.Collect.Memory != nil {
		localCollector.Memory = &troubleshootv1beta2.Memory{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.Memory.CollectorName,
				Exclude:       c.Collect.Memory.Exclude,
			},
		}
	} else if c.Collect.TCPLoadBalancer != nil {
		localCollector.TCPLoadBalancer = &troubleshootv1beta2.TCPLoadBalancer{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.TCPLoadBalancer.CollectorName,
				Exclude:       c.Collect.TCPLoadBalancer.Exclude,
			},
			Address: c.Collect.TCPLoadBalancer.Address,
			Port:    c.Collect.TCPLoadBalancer.Port,
			Timeout: c.Collect.TCPLoadBalancer.Timeout,
		}
	} else if c.Collect.HTTPLoadBalancer != nil {
		localCollector.HTTPLoadBalancer = &troubleshootv1beta2.HTTPLoadBalancer{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.HTTPLoadBalancer.CollectorName,
				Exclude:       c.Collect.HTTPLoadBalancer.Exclude,
			},
			Address: c.Collect.TCPLoadBalancer.Address,
			Port:    c.Collect.TCPLoadBalancer.Port,
			Timeout: c.Collect.TCPLoadBalancer.Timeout,
		}
	} else if c.Collect.DiskUsage != nil {
		localCollector.DiskUsage = &troubleshootv1beta2.DiskUsage{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.DiskUsage.CollectorName,
				Exclude:       c.Collect.DiskUsage.Exclude,
			},
			Path: c.Collect.DiskUsage.Path,
		}
	} else if c.Collect.TCPPortStatus != nil {
		localCollector.TCPPortStatus = &troubleshootv1beta2.TCPPortStatus{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.TCPPortStatus.CollectorName,
				Exclude:       c.Collect.TCPPortStatus.Exclude,
			},
			Interface: c.Collect.TCPPortStatus.Interface,
			Port:      c.Collect.TCPPortStatus.Port,
		}
	} else if c.Collect.HTTP != nil {
		localCollector.HTTP = &troubleshootv1beta2.HostHTTP{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.HTTP.CollectorName,
				Exclude:       c.Collect.HTTP.Exclude,
			},
			Get:  c.Collect.HTTP.Get,
			Post: c.Collect.HTTP.Post,
			Put:  c.Collect.HTTP.Put,
		}
	} else if c.Collect.Time != nil {
		localCollector.Time = &troubleshootv1beta2.HostTime{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.Time.CollectorName,
				Exclude:       c.Collect.Time.Exclude,
			},
		}
	} else if c.Collect.BlockDevices != nil {
		localCollector.BlockDevices = &troubleshootv1beta2.HostBlockDevices{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.BlockDevices.CollectorName,
				Exclude:       c.Collect.BlockDevices.Exclude,
			},
		}
	} else if c.Collect.KernelModules != nil {
		localCollector.KernelModules = &troubleshootv1beta2.HostKernelModules{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.KernelModules.CollectorName,
				Exclude:       c.Collect.KernelModules.Exclude,
			},
		}
	} else if c.Collect.TCPConnect != nil {
		localCollector.TCPConnect = &troubleshootv1beta2.TCPConnect{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.TCPConnect.CollectorName,
				Exclude:       c.Collect.TCPConnect.Exclude,
			},
			Address: c.Collect.TCPConnect.Address,
			Timeout: c.Collect.TCPConnect.Timeout,
		}
	} else if c.Collect.IPV4Interfaces != nil {
		localCollector.IPV4Interfaces = &troubleshootv1beta2.IPV4Interfaces{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.IPV4Interfaces.CollectorName,
				Exclude:       c.Collect.IPV4Interfaces.Exclude,
			},
		}
	} else if c.Collect.FilesystemPerformance != nil {
		localCollector.FilesystemPerformance = &troubleshootv1beta2.FilesystemPerformance{
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
	} else if c.Collect.Certificate != nil {
		localCollector.Certificate = &troubleshootv1beta2.Certificate{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.Certificate.CollectorName,
				Exclude:       c.Collect.Certificate.Exclude,
			},
			CertificatePath: c.Collect.Certificate.CertificatePath,
			KeyPath:         c.Collect.Certificate.KeyPath,
		}
	} else if c.Collect.HostServices != nil {
		localCollector.HostServices = &troubleshootv1beta2.HostServices{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: c.Collect.HostServices.CollectorName,
				Exclude:       c.Collect.HostServices.Exclude,
			},
		}
	} else {
		return nil, errors.New("no spec found to run")
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	result, err := c.RunRemote(ctx, localCollector, remoteCollectorNamePrefix, remoteCollectorDefaultInterval)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run collector remotely")
	}

	if !c.Redact {
		return result, nil
	}

	for path := range result {
		if err := redactResult(path, result, globalRedactors); err != nil {
			// Return result on error to match behaviour of standard collector.
			return result, errors.Wrap(err, "failed to redact")
		}
	}
	return result, nil
}

func (c *RemoteCollector) RunRemote(ctx context.Context, collector *troubleshootv1beta2.HostCollect, namePrefix string, interval time.Duration) (map[string][]byte, error) {
	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, err
	}

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, errors.Wrap(err, "failed to add runtime scheme")
	}

	// Get all the nodes where a Pod should be running
	nodes, err := listNodesInSelectors(ctx, client, c.LabelSelector)
	if err != nil {
		return nil, fmt.Errorf("failed to get the list of nodes matching a nodeSelector, got err: %w", err)
	}

	nameGenerator := names.SimpleNameGenerator

	results := make(map[string][]byte)
	errs := []error{}

	resCh := make(chan map[string][]byte)
	errCh := make(chan error)
	waitCh := make(chan struct{})
	doneCh := make(chan struct{})

	go func() {
		for {
			select {
			case res := <-resCh:
				for k, v := range res {
					results[k] = v
				}
			case err := <-errCh:
				errs = append(errs, err)
			case <-doneCh:
				close(waitCh)
				return
			}
		}
	}()

	var wg sync.WaitGroup
	for _, node := range nodes {
		wg.Add(1)
		go func(nodeName string) {
			name := nameGenerator.GenerateName(namePrefix + "-")
			result, err := run(ctx, client, scheme, collector, c.Image, c.PullPolicy, c.Namespace, name, nodeName, interval)
			if err != nil {
				errCh <- err
			}
			resCh <- result
			wg.Done()
		}(node.Name)
	}

	wg.Wait()
	close(doneCh)
	<-waitCh

	if len(errs) > 0 {
		if len(errs) == len(nodes) {
			return nil, errors.Wrap(errs[0], "Remote collection failed on all nodes")
		}
		logger.Printf("Remote collection failed: %v", err)
	}

	return results, nil
}

func run(ctx context.Context, client *kubernetes.Clientset, scheme *runtime.Scheme, collector *troubleshootv1beta2.HostCollect, image string, pullPolicy string, namespace string, name string, nodeName string, interval time.Duration) (map[string][]byte, error) {
	serviceAccountName := ""
	jobType := "remote-collector"

	cm, pod, err := CreateCollector(client, scheme, nil, name, namespace, nodeName, serviceAccountName, jobType, collector, image, pullPolicy)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create collector")
	}

	defer func() {
		if err := client.CoreV1().Pods(namespace).Delete(context.Background(), pod.Name, metav1.DeleteOptions{}); err != nil {
			logger.Printf("Failed to delete pod %s: %v\n", pod.Name, err)
		}
		if err := client.CoreV1().ConfigMaps(namespace).Delete(context.Background(), cm.Name, metav1.DeleteOptions{}); err != nil {
			logger.Printf("Failed to delete configmap %s: %v\n", pod.Name, err)
		}
	}()

	logs, err := GetContainerLogs(ctx, client, namespace, pod.Name, runnerContainerName, true, interval)
	if err != nil {
		return nil, err
	}

	output := map[string][]byte{
		nodeName: []byte(logs),
	}

	return output, nil
}

func (c *RemoteCollector) GetDisplayName() string {
	return c.Collect.GetName()
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
