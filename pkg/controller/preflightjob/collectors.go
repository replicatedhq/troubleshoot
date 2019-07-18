package preflightjob

import (
	"context"
	"fmt"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	collectrunner "github.com/replicatedhq/troubleshoot/pkg/collect"
)

func (r *ReconcilePreflightJob) reconcilePreflightCollectors(instance *troubleshootv1beta1.PreflightJob, preflight *troubleshootv1beta1.Preflight) error {
	requestedCollectorIDs := make([]string, 0, 0)
	for _, collector := range preflight.Spec.Collectors {
		requestedCollectorIDs = append(requestedCollectorIDs, idForCollector(collector))
		if err := r.reconcileOnePreflightCollector(instance, collector); err != nil {
			return err
		}
	}

	if !contains(requestedCollectorIDs, "cluster-info") {
		clusterInfo := troubleshootv1beta1.Collect{
			ClusterInfo: &troubleshootv1beta1.ClusterInfo{},
		}
		if err := r.reconcileOnePreflightCollector(instance, &clusterInfo); err != nil {
			return err
		}
	}
	if !contains(requestedCollectorIDs, "cluster-resources") {
		clusterResources := troubleshootv1beta1.Collect{
			ClusterResources: &troubleshootv1beta1.ClusterResources{},
		}
		if err := r.reconcileOnePreflightCollector(instance, &clusterResources); err != nil {
			return err
		}
	}

	return nil
}

func (r *ReconcilePreflightJob) reconcileOnePreflightCollector(instance *troubleshootv1beta1.PreflightJob, collect *troubleshootv1beta1.Collect) error {
	if contains(instance.Status.CollectorsRunning, idForCollector(collect)) {
		// preflight just leaves these stopped containers.
		// it's playing with fire a little, but the analyzers can just
		// read from the stdout of the stopped container
		//
		// in the very common use case (what we are building for today)
		// there's not too much risk in something destroying and reaping that stopped pod
		// immediately.  this is a longer term problem to solve, maybe something,
		// the mananger? can broker these collector results.  but, ya know...

		instance.Status.CollectorsSuccessful = append(instance.Status.CollectorsSuccessful, idForCollector(collect))
		instance.Status.CollectorsRunning = remove(instance.Status.CollectorsRunning, idForCollector(collect))

		if err := r.Update(context.Background(), instance); err != nil {
			return err
		}

		return nil
	}

	_, _, err := collectrunner.CreateCollector(r.Client, r.scheme, instance, instance.Name, instance.Namespace, "preflight", collect, instance.Spec.Image, instance.Spec.ImagePullPolicy)
	if err != nil {
		return err
	}

	instance.Status.CollectorsRunning = append(instance.Status.CollectorsRunning, idForCollector(collect))
	if err := r.Update(context.Background(), instance); err != nil {
		return err
	}

	return nil
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func remove(s []string, r string) []string {
	for i, v := range s {
		if v == r {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

// Todo these will overlap with troubleshoot containers running at the same time
func idForCollector(collector *troubleshootv1beta1.Collect) string {
	if collector.ClusterInfo != nil {
		return "cluster-info"
	}
	if collector.ClusterResources != nil {
		return "cluster-resources"
	}
	if collector.Secret != nil {
		return fmt.Sprintf("secret-%s%s", collector.Secret.Namespace, collector.Secret.Name)
	}
	if collector.Logs != nil {
		randomString := "abcdef" // TODO
		return fmt.Sprintf("logs-%s%s", collector.Logs.Namespace, randomString)
	}

	return ""
}
