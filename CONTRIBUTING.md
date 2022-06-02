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
