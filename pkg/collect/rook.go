package collect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/remotecommand"
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

func Rook(ctx *Context) ([]byte, error) {
	client, err := kubernetes.NewForConfig(ctx.ClientConfig)
	if err != nil {
		return nil, errors.New("Failed to create kubernetes clientset")
	}

	namespace := "rook-ceph"
	selector := []string{"app=rook-ceph-operator"}
	pods, podsErrors := listPodsInSelectors(client, namespace, selector)
	if len(podsErrors) > 0 {
		return nil, errors.New("Unable to find pod")
	}

	rookOutput := make(map[string][]byte)
	for _, c := range RookCmds {
		errCh := make(chan error, 1)
		resultCh := make(chan CmdResult, 1)
		go func() {
			// TODO: find pod with 'Running' status, for now use first pod in the array
			// Handles case where multiple pods which match select, i.e. one pod terminating
			stdout, stderr, err := runRookCmd(ctx, client, pods[0], c.Cmd, c.Args)
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

	b, err := json.MarshalIndent(rookOutput, "", "  ")
	if err != nil {
		return nil, err
	}

	return b, err
}

func runRookCmd(ctx *Context, client *kubernetes.Clientset, pod corev1.Pod, cmd, args []string) ([]byte, []byte, error) {
	container := pod.Spec.Containers[0].Name
	req := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec")

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, nil, err
	}

	parameterCodec := runtime.NewParameterCodec(scheme)
	req.VersionedParams(&corev1.PodExecOptions{
		Command:   append(cmd, args...),
		Container: container,
		Stdin:     true,
		Stdout:    false,
		Stderr:    true,
		TTY:       false,
	}, parameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(ctx.ClientConfig, "POST", req.URL())
	if err != nil {
		return nil, nil, err
	}

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: stdout,
		Stderr: stderr,
		Tty:    false,
	})

	if err != nil {
		return nil, nil, err
	}

	return stdout.Bytes(), stderr.Bytes(), nil
}
