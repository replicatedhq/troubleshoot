# Consolidate pod logs
 
## Goals
 
Consolidate all pod log collection to one location so that tools such as [sbctl](https://github.com/replicatedhq/sbctl) can locate them.
 
## Non Goals
 
As a side effect of this change, it may be that the logs collector is entirely deprecated.
 
## Background
 
Currently pod logs are collected by the clusterResources collector from containers for pods that have terminated with an error or are crash-looping, and stored in `/cluster-resources/pods/logs/[namespace]/[pod]/[container].log`.  The logs collector stores each collector’s pod logs in `/[name]/[pod-name]/[container-name].log`, if it is named, or at the root of the support bundle if it is not.  If a pod contains only one container, the file is named `[pod-name].log`.  This means that there are 3-4 different places where logs are to be found.
 
The sbctl tool can only find pod logs if they are stored in the clusterResources directory, and so cannot return any logs collected by the logs collector.
 
Some other collectors, such as Ceph and Longhorn, would benefit from pod log collection but do not appear to have any logic to do so, requiring an additional `logs` collector to be added should we wish to have those logs.

Currently collectors run synchronously.

## High-Level Design

Introduce a new function(s) which does the work of pod log collection, storing those logs where `sbctl` can find them in `/cluster-resources/pods/logs/[namespace]/[pod]/[container].log`.

Switch all collectors to use goroutines to collect (concurrently).

Add a channel to each collector (including `logs`), of a type where collectors can, towards the end of their run, send a slice to the channel specifying the pods that particular collector wants to collect logs from.

The new pod log collection function can read that channel, and trigger collecting logs from any pods that have not already been collected.

Alter the clusterResources collector:

* add an option (default none) to collect pod logs from all pods/containers according to a selector rather than just error pods
* add an option (boolean, default false) to collect logs from all pods/containers within the namespace(s) supplied, or all namespaces if no namespace is listed
* add an option to override the default `MaxLines` (500) currently used in the logs collector
* Add the current `unhealthyPods` slice, plus a slice of the pods found with the new selector option, plus a slice from other collectors (e.g. Ceph, logs collectors) to a new slice, and sort/dedupe it.

Mark the logs collector deprecated, as it duplicates the new functionality added to the clusterResources collector.

Alter the logs collector to simply send the list of pods according to the selector to the new function over the channel.

Ceph & Longhorn collectors:

* add the Ceph or Longhorn namespace pods to the list of pods from which to collect logs.
 
## Detailed Design

New type & function: `podLogCollector` struct, function `GetLogs(channel)`.  The channel is set to receive pods from which to collect logs.

Make `MaxLines` configurable in the collector - take from a parameter in `ClusterResources`.

TODO: `sbctl` has a bug where `kubectl logs` needs `-c` even if the pod has only one container (which `kubectl` does not need in that case on a real k8s cluster)

Tasks that can be split to separate merges (in order):

* Switch all collectors to use goroutines to collect (concurrently).
* Add a channel to all collectors that can be used to send the list of pods.  Create a new function to collect pod logs, that receives on the channel.  Put that in a goroutine. Set a `WaitGroup` that closes the channel when all the collectors (not the new one that grabs pod logs) are completed.
* Change each collector to use the new channel to send the list of pods from which to collect logs (one PR at a time?).  Unit tests can be added/updated at this point.
* Deduplicate the logs collection (if it's not already implemented above)
* Ensure that sbctl no longer needs the `-c`



Example code demonstrating the design:

```go
package main

import (
	"fmt"
	"sync"
	"time"
)

type collector interface {
	Collect(chan []string) string
}

type someCollector struct {
	pods []string
}

func (c *someCollector) Collect(ch chan []string) string {

	fmt.Println("running collect on someCollector")
	time.Sleep(1 * time.Second)
	ch <- c.pods
	return "someCollector finished"
}

type cephCollector struct {
	cephpods []string
}

func (c *cephCollector) Collect(ch chan []string) string {
	fmt.Println("running collect on cephCollector")
	time.Sleep(2 * time.Second)
	ch <- c.cephpods
	return "cephCollector finished"
}

type podLogCollector struct {
}

// function that receives the list of pods to collect logs from
// alter the logs collector to just send a list here, this is a new function
// to go collect the actual logs.
func (p *podLogCollector) GetPodLogs(ch chan []string) string {
	// consolidate all the pods and dedupe
	for pod := range ch {
		time.Sleep(1 * time.Second)
		fmt.Println("podLogCollector collected logs from", pod)
		// add the list to an index
		// if the logs haven't already been collected, go fetch them
	}
	return "podLogCollector finished"
}

func main() {
	podChan := make(chan []string, 2)
	collectors := []collector{
		&someCollector{pods: []string{"pod1", "pod2"}},
		&cephCollector{cephpods: []string{"rook1", "osd1", "mon1"}},
		&someCollector{pods: []string{"pod3", "pod4"}},
		&someCollector{pods: []string{"pod5", "pod6"}},
	}

	var logsWg sync.WaitGroup
	logsWg.Add(1)
	// grab a list of pods from which to collect logs via a channel read
	logs := &podLogCollector{}
	go func(l *podLogCollector) {
		fmt.Println("Running GetPodLogs.... ", l.GetPodLogs(podChan))
		defer logsWg.Done()
	}(logs)

	// run the collectors, they need to send the pod list which wants logs to the channel
	var wg sync.WaitGroup
	for _, coll := range collectors {
		wg.Add(1)
		go func(coll collector) {
			defer wg.Done()
			fmt.Println(coll.Collect(podChan))
		}(coll)
	}

	wg.Wait()
	close(podChan)
	logsWg.Wait()
}
```

## Limitations
 
## Assumptions

* All collectors can be run concurrently, none depend on other collectors
* The Kubernetes API can handle the load of collectors running concurrently, including when the pod logs collector runs
 
## Testing

The new function will need unit tests.

The existing Collect functions will need unit tests altered (or added).

The concept for this change was tested using the example code above.

## Alternatives Considered

Two alternatives were considered:

* Add a switch to `clusterResources` to enable collection from selected, or all, pods.  
* Change the location which the `logs` collector uses to be the same as `clusterResources`.

Both have similar benefits and drawbacks.

Pros: 

* Simple, minimal changes to the existing code.
* Does not rely on earlier changes to collectors to have them implement an interface.

Cons:

* Does not allow for "other" collectors (e.g. Ceph) to supply a list of pods to add to the selector.  This means we would end up in an "all or nothing" situation or have a long list of potential selectors either added by default, or added by folks using the tool.

## Security Considerations

None identified.