**Managing Troubleshoot CRDs**

The CRDs in this folder are used by Troubleshoot in combination with the spec that can be passed on to the Troubleshoot CLI in various ways (the location of the spec can be passed on as a web address, yaml file or a secret residing on a cluster).

Should the CRDs (and corresponding schemas) need to be (re-) generated or if an additional field or type needs to be added, the following steps need to be taken;

**Adding new field(s)/type(s):**

New fields (types) need to be defined in the `./pkg/apis/troubleshoot/v1beta2/supportbundle_types.go` file.


**(Re)generate new schema's/CRDs:**

Generating new schemas in `./schemas` can be done by running `make schemas`, which will then implement the new type into the CRDs

*TBD: More detailed documentation on the CRDs and corresponding components.*
