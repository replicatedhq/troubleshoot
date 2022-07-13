# Contributing to Troubleshoot

Thank you for your interest in Troubleshoot, we welcome your participation. Please familiarize yourself with our [Code of Conduct](https://github.com/replicatedhq/troubleshoot/blob/master/CODE_OF_CONDUCT.md) prior to contributing. There are a number of ways to participate in Troubleshoot as outlined below:

## Issues
- [Request a New Feature](https://github.com/replicatedhq/troubleshoot/issues/new?assignees=&labels=feature&template=feature_enhancement.md) Create an issue to add functionality that addresses a problem or adds an enhancement.  
- [Report a Bug](https://github.com/replicatedhq/troubleshoot/issues/new?assignees=&labels=bug&template=bug_report.md) Report a problem or unexpected behaviour with Troubleshoot. 

## Development Environment

To get started we recommend:

1. Go (v1.17 or later)
2. A Kubernetes cluster (we recommend https://k3d.io/. This requires Docker v20.10.5 or later)
3. Fork and clone the repo to $GOPATH/src/github.com/replicatedhq/
4. Run `make support-bundle preflight` to generate binaries
5. Run `make run-troubleshoot` to generate a support bundle with the `sample-troubleshoot.yaml` in the root of the repo

### Testing

To run the tests locally run the following:

```bash
make test
```

Additionally, e2e tests can be run with:

```bash
make support-bundle preflight e2e-test
```

A Kubernetes cluster as well as `jq` are required to run e2e tests.

## Contribution workflow

This is a rough outline of how to prepare a contribution:

- Create a topic branch from where you want to base your work (branched from main).
- Make commits of logical units.
- Push your changes to a topic branch in your fork of the repository.
- Submit a pull request to the original repository. It will be reviewed in a timely manner.

### Pull Requests

A pull request should address a single issue, feature or bug. For example, lets say you've written code that fixes two issues. That's great! However, you should submit two small pull requests, one for each issue as opposed to combining them into a single larger pull request. In general the size of the pull request should be kept small in order to make it easy for a reviewer to understand, and to minimize risks from integrating many changes at the same time. For example, if you are working on a large feature you should break it into several smaller PRs by implementing the feature as changes to several packages and submitting a separate pull request for each one.

Code submitted in pull requests must be properly documented, formatted and tested in order to be approved and merged. The following guidelines describe the things a reviewer will look for when they evaluate your pull request. Here's a tip. If your reviewer doesn't understand what the code is doing, they won't approve the pull request. Strive to make code clear and well documented. If possible, request a reviewer that has some context on the PR.
