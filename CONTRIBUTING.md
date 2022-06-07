# Contributing to Troubleshoot

Thank you for your interest in Troubleshoot, we welcome your participation. Please familiarize yourself with our [Code of Conduct](https://github.com/replicatedhq/troubleshoot/blob/master/CODE_OF_CONDUCT.md) prior to contributing. There are a number of ways to participate in Troubleshoot as outlined below:

## Issues
- [Request a New Feature](https://github.com/replicatedhq/troubleshoot/issues/new?assignees=&labels=feature&template=feature_enhancement.md) Create an issue to add functionality that addresses a problem or adds an enhancement.
- [Report a Bug](https://github.com/replicatedhq/troubleshoot/issues/new?assignees=&labels=bug&template=bug_report.md) Report a problem or unexpected behaviour with Troubleshoot.

## Pull Requests

If you are interested in contributing a change to the code or documentation please open a pull request with your set of changes. The pull request will be reviewed in a timely manner.

## Tests

To run the tests locally run the following:

```bash
make test
```

Additionally, e2e tests can be run with:

```bash
make support-bundle preflight e2e-test
```

A kubernetes cluster as well as `jq` are required to run e2e tests.

## Local development

Before you start, you will need to ensure you have the following installed on your laptop:

* Docker for desktop
* Go (1.17)
* Kubectl

Clone the [Troubleshoot.sh code](https://github.com/replicatedhq/troubleshoot)
and check out your branch (or `main`).

You will need a Kubernetes cluster running and accessible.  For this exercise, a k3d cluster is enough, but you might have something else accessible that would potentially give you more functionality.  As a minimum, you want to be running kubectl commands to get cluster info, and be able to create a pod to run commands.

Example k3d setup:

```
$ curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash
[sudo] password for xav:
k3d installed into /usr/local/bin/k3d

Run 'k3d --help' to see what you can do with it
```

Make a new cluster:

```
$ k3d cluster create mycluster

INFO[0000] Prep: Network
<snip lots of output>
INFO[0035] Cluster 'mycluster' created successfully!
INFO[0035] You can now use it like this:
kubectl cluster-info
```

Build the code from the root directory of your branch:

```
$ make support-bundle
go build -tags "netgo containers_image_ostree_stub exclude_graphdriver_devicemapper exclude_graphdriver_btrfs containers_image_openpgp" -installsuffix netgo -ldflags " -s -w -X github.com/replicatedhq/troubleshoot/pkg/version.version=`git describe --tags --dirty` -X github.com/replicatedhq/troubleshoot/pkg/version.gitSHA=`git rev-parse HEAD` -X github.com/replicatedhq/troubleshoot/pkg/version.buildTime=`date -u +"%Y-%m-%dT%H:%M:%SZ"` " -o bin/support-bundle github.com/replicatedhq/troubleshoot/cmd/troubleshoot
```

You can also run `make preflight` and `make collect` to make those binaries, and `make test` to run tests.

Make a sample bundle to collect - there are examples in the docs, but here is a very basic starting point with an analyzer we expect to fail:

```yaml
apiVersion: troubleshoot.sh/v1beta2
kind: Collector
metadata:
  name: my-application-name
spec:
  collectors:
  - clusterInfo:
      collectorName: my-cluster-info
  - clusterResources:
      collectorName: my-cluster-resources
  - http:
      name: google
      get:
        url: https://www.google.com
  analyzers:
    - storageClass:
        checkName: Required storage classes
        storageClassName: "microk8s-hostpath"
        outcomes:
          - fail:
              message: The required storage class was not found in the cluster.
          - pass:
              message: The required storage class was found in the cluster.
```

Run the support-bundle binary:

```
$ ./bin/support-bundle tshoot.yaml

Collecting support bundle ⠸ cluster-resourcesW0603 13:24:54.525201 3252874 warnings.go:70] batch/v1beta1 CronJob is deprecated in v1.21+, unavailable in v1.25+; use batch/v1 CronJob
W0603 13:24:54.534853 3252874 warnings.go:70] batch/v1beta1 CronJob is deprecated in v1.21+, unavailable in v1.25+; use batch/v1 CronJob
Collecting support bundle ⠼ cluster-resourcesW0603 13:24:54.544404 3252874 warnings.go:70] batch/v1beta1 CronJob is deprecated in v1.21+, unavailable in v1.25+; use batch/v1 CronJob
W0603 13:24:54.551911 3252874 warnings.go:70] batch/v1beta1 CronJob is deprecated in v1.21+, unavailable in v1.25+; use batch/v1 CronJob
Collecting support bundle ⠼ cluster-resourcesI0603 13:24:56.553583 3252874 request.go:665] Waited for 1.195168019s due to client-side throttling, not priority and fairness, request: GET:https://0.0.0.0:37173/apis/events.k8s.io/v1beta1
Collecting support bundle ⠸ cluster-resourcesI0603 13:25:06.553757 3252874 request.go:665] Waited for 4.288870241s due to client-side throttling, not priority and fairness, request: GET:https://0.0.0.0:37173/apis/flowcontrol.apiserver.k8s.io/v1beta1
Collecting support bundle ⠏ httpsupport-bundle-2022-06-03T13_24_54.tar.gz
```

What you have now, is a bundle file `support-bundle-2022-06-03T13_24_54.tar.gz` you can work with.

You’re going to want to look at this bundle, however a lot of it is json rather than simple to parse output that we like to read as human beings.  This is where [sbctl](https://github.com/replicatedhq/sbctl/releases) comes in.  Build/install that according to the instructions in the repo.

An example of how to use this:

```
$ sbctl serve --support-bundle-location=./support-bundle-2022-06-03T13_24_54.tar.gz
INFO[0000] called getAPIV1INFO[0000] 200 GET /api/v1Server is running

export KUBECONFIG=/tmp/local-kubeconfig-2216478198
```

You can now use kubectl to review things:

```
$ export KUBECONFIG=/tmp/local-kubeconfig-2216478198
$ kubectl get po -A
NAMESPACE     NAME                                      READY   STATUS      RESTARTS   AGE
kube-system   local-path-provisioner-84bb864455-f9n54   1/1     Running     0          17m
kube-system   coredns-96cc4f57d-svrsf                   1/1     Running     0          17m
kube-system   helm-install-traefik-crd--1-zprvp         0/1     Completed   0          17m
kube-system   metrics-server-ff9dbcb6c-gbxzm            1/1     Running     0          17m
kube-system   helm-install-traefik--1-m2wxv             0/1     Completed   2          17m
kube-system   svclb-traefik-rtd2m                       2/2     Running     0          16m
kube-system   traefik-56c4b88c4b-tlsfc                  1/1     Running     0          16m
```

An important note to remember is that a cluster represented by sbctl is basically a read-only snapshot of a k8s cluster. This means that you can’t change anything to test if this will resolve a known issue.

