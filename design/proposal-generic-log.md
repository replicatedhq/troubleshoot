# Implement a generic log collector for all others to use
 
## Goals

Consolidate all pod log collection to one location so that tools such as [sbctl](https://github.com/replicatedhq/sbctl) can locate them.

Allow collectors other than `clusterResources` and `logs` to collect pod logs on demand, without duplicating code.

## Non Goals

This change does not replace, or deprecate, the `logs` collector, but does change the focus of it to simply provide a list to another collector to do the work.

## Background

Currently two separate collectors collect pod logs.  They store the data in different places, and have no chance of preventing a pod's logs from being collected and stored more than once.

The two collectors, `clusterResources` and `logs` both have different collection code for pod logs.

## High-Level Design

Introduce a new function(s) which does the work of pod log collection, storing those logs where `sbctl` can find them in `/cluster-resources/pods/logs/[namespace]/[pod]/[container].log`.  This should start in a goroutine, prior to any collectors being run.

Add a channel to `SupportBundleCreateOpts` that collectors can, when `.Collect()` is called, send a slice to the channel specifying the pods that particular collector wants to collect logs from.  For the `logs` and `clusterResources` collectors, this replaces the actual pod log collection routine.

The new pod log collection function can read that channel, and trigger collecting logs from any pods that have not already been collected.

The pod log collection function runns in a goroutine, and is terminated only once all the other collectors are finished.

Ceph & Longhorn collectors should be modified to add the Ceph or Longhorn namespace pods to the list of pods from which to collect logs.
 
## Detailed Design

New type & function: `podLogCollector` struct, function `GetLogs(channel)`.  The channel is set to receive pods from which to collect logs.

Example code broadly demonstrating the design:

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

* The Kubernetes API can handle the load of collectors running concurrently, including when the pod logs collector runs
 
## Testing

The new function will need unit tests.

The existing Collect functions will need unit tests altered (or added).

## Documentation

No changes required.

## Alternatives Considered

* Having each collector append the list of pods to a global, then on completion of all collectors run the collector for pod logs (starting with deduplicating the list of pods).  This would be slower since we need to wait for all collectors to complete prior to pod log collection starting.
* Making a generic logs collector function that can be imported by any collector that wishes to collect logs.  This would not allow the same pod's logs to be collected several times.

## Security Considerations

None identified.