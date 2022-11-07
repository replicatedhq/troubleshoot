# ADR 001: remove IP address redaction by default

## Context

Since PR #8 IP addresses have automatically been redacted throughout support bundles.  This was originally
added so that folks with security requirements that include not sharing IP addresses can avoid needing to add
individual redactors for that purpose.

The general requirement for environments that protect network infrastructure from being communicated outside also
includes hostnames, port numbers, and mac addresses.  The Troubleshoot code does not redact these.

The experience of support engineers making use of Troubleshoot support bundles has been that in many cases,
a further set of logs, and data collection information needs to be collected for issues that involve network
infrastructure in order to be able to assist.  This involves collection that does not redact IP addresses, and
introduces significant delays in the resolution of issues due to back and forth.

A review of Replicated support cases where end users posted an IP address in GitHub found 71 different issues
in 2022 (as of October 2022). This doesn't cover uploaded files, phone calls, screen shares, etc. It also
doesn't include anything else networking like mac address, hostnames, ports.

The problem discussed in this document:

* Users of Troubleshoot need to avoid redaction if they have networking issues, in order to share IP addresses
  that are redacted by default.
* The user experience of the product is based around "define what you want", however with default redactors built
  into the product in addition to the defined spec is confusing and, in some cases, problematic if you do not want
  the built in redactors.
* A default set of redactors gives product users a false sense of security in that though the default redactors
  do cover some redaction, they do not cover every possible combination for every particular sensitive item.
  Redaction of hostnames is almost impossible given the freeform nature of hostnames, and the built in Password
  redactor does not cover all passwords, merely a particular json combination.

## Decision

We will remove the IP address redactor from Troubleshoot.

## Solution

The following changes need to be made:

* [Documentation](https://troubleshoot.sh/docs/redact/ip-addresses/) for the IP address redaction needs altering to reflect that Troubleshoot
  does not automatically redact IP addresses, but if users wish to there is an example yaml spec available.
* Removal of the IP address redactor from [the code](https://github.com/replicatedhq/troubleshoot/blob/v0.45.0/pkg/redact/redact.go#L170)
* Clear release notes on release of this change, communicating that those wishing to redact IP addresses need to add that redactor to their code.
* Possible broadcast communication since this is a change to default behavior.

## Status

Proposed

## Consequences

Describe the resulting context, after applying the decision. All consequences should be listed here,
not just the "positive" ones. A particular decision may have positive, negative, and neutral consequences,
but all of them affect the team and project in the future.

Those that wish to have IP addresses redacted by Troubleshoot need to ensure that the redactor specified during `support-bundle` runs includes a
regex for IP address redaction.

Example regex for IP redaction:

```yaml
apiVersion: troubleshoot.sh/v1beta2
kind: Redactor
metadata:
  name: IP Addresses
spec:
  redactors:
  - name: Redact ipv4 addresses
    removals:
      regex:
      - redactor: '(?P<mask>\b(?P<drop>25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(?P<drop>25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(?P<drop>25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(?P<drop>25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b)'
```

Folks that have not specified the IP address regex above will, on support bundle creation, share their IP address information in that bundle by default.