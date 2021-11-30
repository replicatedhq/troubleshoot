package hack

import (
	// hack to make sure package is not pruned by go mod tidy
	_ "k8s.io/code-generator/pkg/util"
	_ "sigs.k8s.io/controller-tools/pkg/version"
)
