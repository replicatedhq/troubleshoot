# enable cluster-resources abstraction for existing analyzers
 
## Goals

Remove required knowlege of file path from existing analyzers when targeting resources collected by the cluster-resources collector

## Non Goals

adding new analyzers

## Background

Currently a user wanting to analyze a resource collected by the cluster-resources collector have to manually specify the file location in one of the existing collectors.

## High-Level Design

This proposal suggests adding a new field to the collectors that need easy access to cluster-resource files such as the text analyzer, jsonpath analyzer or yaml analizer to utilize the recently merged FindResource() function.

## Detailed Design

currently FindResource() returns an interface{}, but it would be trivial since it's currently only implimented in an example collector to change it to return a `[]byte` or `map[string][]byte` and use it as a conditional drop-in for os.ReadFile() or similar.

it can then be leveraged from other aanalyzers upon specifying a `clusterResource:` field in the spec.

An example of how an existing check for a cluster-resource file looks now:

```yaml
  analyzers:
    - textAnalyze:
        checkName: Database Authentication
        fileName: cluster-resources/deployments/default.json
        regex: 'image: reg/someimage'
        outcomes:
          - pass:
              when: "false"
              message: "image ok"
          - fail:
              when: "true"
              message: "image not ok"
```

this is problematic because it captures every resource in the default namespace, and makes getting to the subset of the information you actually want to analyze hard if not impossible.

This proposal suggests the new `clusterResource` field be used optionally in place of `fileName`, so the original functionality remains for checking non cluster-resources files:

```yaml
  analyzers:
    - textAnalyze:
        checkName: Database Authentication
        clusterResource:
            Kind: Deployment
            Namespace: Default
            Name: somedeployment
        regex: 'image: reg/someimage'
        outcomes:
          - pass:
              when: "false"
              message: "image ok"
          - fail:
              when: "true"
              message: "image not ok"
```

in the analyzer code itself we can switch on weather `filename` or `clusterResource` was defined and use either the existing `getCollectedFileContents()` or `FindResource()`

## Limitations



## Assumptions



## Testing

the analyzers that impliment this function may need new test cases

## Documentation

The Documentation for each alayzer that impliments `clusterResources` or `FindResource` needs to be updated to state that it can't be used at the same time as `fileName`, as well as the selector options for `clusterResource`

## Alternatives Considered


## Security Considerations


