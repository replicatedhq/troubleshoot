# Contributing to Troubleshoot

Thank you for your interest in Troubleshoot, we welcome your participation. Please familiarize yourself with our [Code of Conduct](https://github.com/replicatedhq/troubleshoot/blob/main/CODE_OF_CONDUCT.md) prior to contributing. There are a number of ways to participate in Troubleshoot as outlined below:

# Community

For discussions about developing Troubleshoot, there's an [#app-troubleshoot channel in Kubernetes Slack](https://kubernetes.slack.com/channels/app-troubleshoot), plus IRC using [Libera](ircs://irc.libera.chat:6697/#troubleshoot) (#troubleshoot).

There are [community meetings](https://calendar.google.com/calendar/u/0?cid=Y19mMGx1aGhiZGtscGllOGo5dWpicXMwNnN1a0Bncm91cC5jYWxlbmRhci5nb29nbGUuY29t) on a regular basis, with a shared calendar and [public notes](https://hackmd.io/yZbotEHdTg6TfRZBzb8Tcg)

## Issues

- [Request a New Feature](https://github.com/replicatedhq/troubleshoot/issues/new?assignees=&labels=feature&template=feature_enhancement.md) Create an issue to add functionality that addresses a problem or adds an enhancement.
- [Report a Bug](https://github.com/replicatedhq/troubleshoot/issues/new?assignees=&labels=bug&template=bug_report.md) Report a problem or unexpected behaviour with Troubleshoot.

## Design Principles

When implementing a new feature please review the [design principles](./design/design-principles.md) to help guide the approach.

## Development Environment

To get started we recommend:

1. Go (v1.19 or later)
2. A Kubernetes cluster (we recommend <https://k3d.io/>. This requires Docker v20.10.5 or later)
3. Fork and clone the repo to $GOPATH/src/github.com/replicatedhq/
4. Run `make support-bundle preflight` to generate binaries
5. Run `make run-troubleshoot` to generate a support bundle with the `sample-troubleshoot.yaml` in the root of the repo

> Note: recent versions of Go support easy cross-compilation.  For example, to cross-compile a Linux binary from MacOS:
> `GOOS=linux GOARCH=amd64 make support-bundle preflight`

6. Install [golangci-lint] linter and run `make lint` to execute additional code linters.

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

- Create a fork of this repo.
- Create a topic branch from where you want to base your work (branched from `main` is a safe choice).
- Make commits of logical units.
- When your changes are ready to merge, squash your history to 1 commit.
  - For example, if you want to squash your last 3 commits and write a new commit message:
      ```
      git reset --soft HEAD~3 &&
      git commit
      ```

  - If you want to keep the previous commit messages and concatenate them all into a new commit, you can do something like this instead:
      ```
      git reset --soft HEAD~3 &&
      git commit --edit -m"$(git log --format=%B --reverse HEAD..HEAD@{1})"
      ```
- Push your changes to a topic branch in your fork of the repository.
- Submit a pull request to the original repository. It will be reviewed in a timely manner.

### Pull Requests

A pull request should address a single issue, feature or bug. For example, lets say you've written code that fixes two issues. That's great! However, you should submit two small pull requests, one for each issue as opposed to combining them into a single larger pull request. In general the size of the pull request should be kept small in order to make it easy for a reviewer to understand, and to minimize risks from integrating many changes at the same time. For example, if you are working on a large feature you should break it into several smaller PRs by implementing the feature as changes to several packages and submitting a separate pull request for each one.  Squash commit history when preparing your PR so it merges as 1 commit.

Code submitted in pull requests must be properly documented, formatted and tested in order to be approved and merged. The following guidelines describe the things a reviewer will look for when they evaluate your pull request. Here's a tip. If your reviewer doesn't understand what the code is doing, they won't approve the pull request. Strive to make code clear and well documented. If possible, request a reviewer that has some context on the PR.

### Commit messages

Commit messages should follow the general guidelines:

- Breaking changes should be highlighted in the heading of the commit message.
- Commits should be clear about their purpose (and a single commit per thing that changed)
- Messages should be descriptive:
  - First line, 50 chars or less, as a heading/title that people can find
  - Then a paragraph explaining things
- Consider a footer with links to which bugs they fix etc, bearing in mind that Github does some of this magic already