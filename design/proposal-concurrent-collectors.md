# Run collectors concurrently
 
## Goals

Increase the speed at which collectors complete.

Consolidate all pod log collection to one location so that tools such as [sbctl](https://github.com/replicatedhq/sbctl) can locate them.

Ensure that the cluster-resources collector runs prior to any others, so that things like pod status and pod logs only include the pods from the application rather than the ones that Troubleshoot started as part of collectors.

Ensure that when needed, a collector can ensure that another collector has run prior to starting without needing to rely on the ordering in a spec.

## Non Goals

A large portion of this change involves the logs collector, while this is not the primary goal it does allow collectors to trigger pod log collection from one place.
 
## Background

Currently all collectors run sequentially, in the order in which they are specified in the spec.  With the addition of the ability to supply multiple specs, the order is no longer guaranteed.

The collector code includes a check to ensure that `clusterResources` and `clusterInfo` are added to every collection regardless of the spec.  These are added at the end of the run, and so collect data for pods created during the Troubleshoot collection, which causes poor output results (see [#767](https://github.com/replicatedhq/troubleshoot/issues/767)).

## High-Level Design

Introduce a new parameter for `supportBundle` spec collectors: `dependsOn`.  This is to be used to ensure that a specific collector will only run once the one specified is completed.  e.g.:

```yaml
    - exec:
        name: generate-output
        collectorName: generate-output
        namespace: '{{repl Namespace}}'
        selector:
          - app=some-selector
        command: ["sh"]
        args: ["-c", "cd /some/data/dir && mv `ls -t | head -n 1` latest"]
        timeout: 5s
    - copy:
        name: collect-output
        dependsOn: generate-output
        collectorName: collect-output
        selector:
          - app=some-selector
        namespace: '{{repl Namespace}}'
        containerPath: /some/data/dir/latest
```

This concept requires

* error handling (what if that collector isn't in the spec, or fails)
* logic to order all the collectors that depend on others as there might be several in the line

Introduce a new function(s) which does the work of pod log collection, storing those logs where `sbctl` can find them in `/cluster-resources/pods/logs/[namespace]/[pod]/[container].log`.  This should start in a goroutine, prior to any collectors being run.

Switch all collectors to use goroutines to collect (concurrently).  This should be written such that collectors are split into separate lists, in order that any collectors with `dependsOn` are in a phase later than the collector it depends on.  Run each phase in turn, at this stage it may be acceptable to wait for the entire list of collectors in that phase to complete prior to starting the next phase.

Add a channel to each collector (including `logs`), of a type where collectors can, towards the end of their run, send a slice to the channel specifying the pods that particular collector wants to collect logs from.  For the `logs` and `clusterResources` collectors, this replaces the actual pod log collection routine.

The new pod log collection function can read that channel, and trigger collecting logs from any pods that have not already been collected.

The pod log collection function runns in a goroutine, and is terminated only once all the other collectors are finished.

Ceph & Longhorn collectors:

* add the Ceph or Longhorn namespace pods to the list of pods from which to collect logs.

`clusterResources` collector:

Ensure that `clusterResources` runs prior to all others.
 
## Detailed Design

New type & function: `podLogCollector` struct, function `GetLogs(channel)`.  The channel is set to receive pods from which to collect logs.

Make `MaxLines` configurable in the collector - take from a parameter in `ClusterResources`.

TODO: `sbctl` has a bug where `kubectl logs` needs `-c` even if the pod has only one container (which `kubectl` does not need in that case on a real k8s cluster)

Tasks that can be split to separate merges (in order):

* Switch all collectors to use goroutines to collect (concurrently).
* Introduce a new collector parameter `dependsOn`, and code to run the collectors concurrently in batches so that the `dependsOn` is honored.  This could be via a separate channel where collectors wait for a condition, or in batches/phases where all collectors in one round are completed prior to starting the next round.
* Add a channel to all collectors that can be used to send the list of pods.  Create a new function to collect pod logs, that receives on the channel.  Put that in a goroutine. Set a `WaitGroup` that closes the channel when all the collectors (not the new one that grabs pod logs) are completed.
* Change each collector to use the new channel to send the list of pods from which to collect logs (one PR at a time?).  Unit tests can be added/updated at this point.
* Deduplicate the logs collection (if it's not already implemented above)
* Ensure that sbctl no longer needs the `-c`

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

The concept for this change was tested using the example code above.

## Documentation

This requires updates to the docs for the new `dependsOn` field.

## Alternatives Considered

Introduce the concept of 'phases' for collection.

* clusterResources and clusterInfo run in phase 0.  Nothing else runs in this phase.
* By default everything with no phase set runs next, phase 1.
* A collector can be configured to have a phase other than 1, e.g. 2, which means it will run after phase 1 collectors, and before phase 3 collectors.
* Troubleshoot will continue running each set of phases till there are no more.

This would be simpler to write into code, but comes at the cost of usability and clarity in the supportBundle spec.

## Security Considerations

None identified.