package provisioner

import (
	"context"

	"github.com/supergiant/supergiant/pkg/profile"
	"github.com/supergiant/supergiant/pkg/workflows"
	"github.com/supergiant/supergiant/pkg/node"
	"github.com/supergiant/supergiant/pkg/storage"
	"github.com/supergiant/supergiant/pkg/workflows/steps"
	"io/ioutil"
)

// Provisioner gets kube profile and returns list of task ids of provision tasks
type Provisioner interface {
	Provision(ctx context.Context, kubeProfile profile.KubeProfile)
	Cancel()
}

type TaskProvisioner struct{
	repository storage.Interface


	tasksIds []string
	cancelFuncs []func()
}

// Provision runs provision process among nodes that have been provided for provision
func (r *TaskProvisioner) Provision(ctx context.Context, nodes []node.Node) ([]string, error) {
	r.cancelFuncs = make([]func(), 0, len(nodes))
	r.tasksIds = make([]string, 0, len(nodes))

	for _, n := range nodes {
		c, cancel := context.WithCancel(ctx)
		r.cancelFuncs = append(r.cancelFuncs, cancel)
		config := steps.Config{}
		t, err := workflows.NewTask(n.Role, config, r.repository)

		if err != nil {
			return nil, err
		}

		// TODO(stgleb): pass buffer here
		t.Run(c, ioutil.Discard)
		r.tasksIds = append(r.tasksIds, t.Id)
	}

	return r.tasksIds, nil
}

// Cancel call cancel funcs of all context of all tasks
func (r *TaskProvisioner) Cancel() {
	for _, cancel := range r.cancelFuncs {
		cancel()
	}
}
