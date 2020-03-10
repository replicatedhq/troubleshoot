package collect

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"k8s.io/client-go/kubernetes"
)

type CmdResult struct {
	Stdout []byte
	Stderr []byte
}

type RookCmd struct {
	Cmd      []string
	Args     []string
	Filename string // file in the support-bundle
}

var RookCmds = []RookCmd{
	RookCmd{
		Cmd:      []string{"ceph", "status"},
		Args:     []string{"-f", "json-pretty"},
		Filename: "status",
	},
	RookCmd{
		Cmd:      []string{"ceph", "fs", "status"},
		Args:     []string{"-f", "json-pretty"},
		Filename: "fs",
	},
	RookCmd{
		Cmd:      []string{"ceph", "fs", "ls"},
		Args:     []string{"-f", "json-pretty"},
		Filename: "fs-ls",
	},
	RookCmd{
		Cmd:      []string{"ceph", "osd", "status"},
		Args:     []string{"-f", "json-pretty"},
		Filename: "osd-status",
	},
	RookCmd{
		Cmd:      []string{"ceph", "osd", "tree"},
		Args:     []string{"-f", "json-pretty"},
		Filename: "osd-tree",
	},
	RookCmd{
		Cmd:      []string{"ceph", "osd", "pool", "ls", "detail"},
		Args:     []string{"-f", "json-pretty"},
		Filename: "osd-pool",
	},
	RookCmd{
		Cmd:      []string{"ceph", "health", "detail"},
		Args:     []string{"-f", "json-pretty"},
		Filename: "health",
	},
	RookCmd{
		Cmd:      []string{"ceph", "auth", "ls"},
		Args:     []string{"-f", "json-pretty"},
		Filename: "auth",
	},
}

func Rook(ctx *Context, rookCollector *troubleshootv1beta1.Rook) ([]byte, error) {
	client, err := kubernetes.NewForConfig(ctx.ClientConfig)
	if err != nil {
		return nil, errors.New("Failed to create kubernetes clientset")
	}

	namespace := rookCollector.Namespace
	if namespace == "" {
		namespace = "rook-ceph"
	}

	selector := rookCollector.Selector
	if len(selector) == 0 {
		// kURL labels ceph operator pod as 'rook-ceph-operator'
		selector = []string{"app=rook-ceph-operator"}
	}

	pods, podsErrors := listPodsInSelectors(client, namespace, selector)
	if len(podsErrors) > 0 {
		return nil, errors.New(strings.Join(podsErrors, " "))
	}

	rookOutput := make(map[string][]byte)
	if len(pods) > 0 {
		container := pods[0].Spec.Containers[0].Name
		if rookCollector.ContainerName != "" {
			container = rookCollector.ContainerName
		}

		for _, c := range RookCmds {
			errCh := make(chan error, 1)
			resultCh := make(chan CmdResult, 1)
			go func() {
				// TODO: find pod with 'Running' status, for now use first pod in the array
				// Handles case where multiple pods which match select, i.e. one pod terminating
				stdout, stderr, err := execPodCmd(ctx, client, pods[0], container, c.Cmd, c.Args)
				if err != nil {
					errCh <- err
				} else {
					resultCh <- CmdResult{Stdout: stdout, Stderr: stderr}
				}
			}()

			select {
			case <-time.After(10 * time.Second):
				err := errors.New("Cmd timed out")
				rookOutput[fmt.Sprintf("rook/status/%s-errors.json", c.Filename)] = []byte(err.Error())
			case result := <-resultCh:
				rookOutput[fmt.Sprintf("rook/status/%s.json", c.Filename)] = result.Stdout
				if len(result.Stderr) > 0 {
					rookOutput[fmt.Sprintf("rook/status/%s-errors.json", c.Filename)] = result.Stderr
				}
			case err := <-errCh:
				rookOutput[fmt.Sprintf("rook/status/%s-errors.json", c.Filename)] = []byte(err.Error())
			}
		}
	}

	b, err := json.MarshalIndent(rookOutput, "", "  ")
	if err != nil {
		return nil, err
	}

	return b, err
}
