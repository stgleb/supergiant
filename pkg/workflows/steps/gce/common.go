package gce

import (
	"context"
	"encoding/json"
	"time"

	"github.com/pkg/errors"

	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"

	"github.com/supergiant/control/pkg/workflows/steps"
)

type computeService struct {
	getFromFamily       func(context.Context, steps.GCEConfig) (*compute.Image, error)
	getMachineTypes     func(context.Context, steps.GCEConfig) (*compute.MachineType, error)
	insertInstance      func(context.Context, steps.GCEConfig, *compute.Instance) (*compute.Operation, error)
	getInstance         func(context.Context, steps.GCEConfig, string) (*compute.Instance, error)
	setInstanceMetadata func(context.Context, steps.GCEConfig, string, *compute.Metadata) (*compute.Operation, error)
	deleteInstance      func(string, string, string) (*compute.Operation, error)

	insertTargetPool           func(context.Context, steps.GCEConfig, *compute.TargetPool) (*compute.Operation, error)
	insertAddress              func(context.Context, steps.GCEConfig, *compute.Address) (*compute.Operation, error)
	getAddress                 func(context.Context, steps.GCEConfig, string) (*compute.Address, error)
	insertForwardingRule       func(context.Context, steps.GCEConfig, *compute.ForwardingRule) (*compute.Operation, error)
	addInstanceToTargetGroup   func(context.Context, steps.GCEConfig, string, *compute.TargetPoolsAddInstanceRequest) (*compute.Operation, error)
	addInstanceToInstanceGroup func(context.Context, steps.GCEConfig, string, *compute.InstanceGroupsAddInstancesRequest) (*compute.Operation, error)
	insertInstanceGroup        func(context.Context, steps.GCEConfig, *compute.InstanceGroup) (*compute.Operation, error)
	insertBackendService       func(context.Context, steps.GCEConfig, *compute.BackendService) (*compute.Operation, error)
	insertNetwork              func(context.Context, steps.GCEConfig, *compute.Network) (*compute.Operation, error)
	getInstanceGroup           func(context.Context, steps.GCEConfig, string) (*compute.InstanceGroup, error)
	getTargetPool              func(context.Context, steps.GCEConfig, string) (*compute.TargetPool, error)
	getBackendService          func(context.Context, steps.GCEConfig, string) (*compute.BackendService, error)
	deleteForwardingRule       func(context.Context, steps.GCEConfig, string) (*compute.Operation, error)
	deleteBackendService       func(context.Context, steps.GCEConfig, string) (*compute.Operation, error)
	deleteInstanceGroup        func(context.Context, steps.GCEConfig, string) (*compute.Operation, error)
	deleteTargetPool           func(context.Context, steps.GCEConfig, string) (*compute.Operation, error)
	deleteIpAddress            func(context.Context, steps.GCEConfig, string) (*compute.Operation, error)
}

func Init() {
	createInstance := NewCreateInstanceStep(time.Second*10, time.Minute*5)
	deleteCluster := NewDeleteClusterStep()
	deleteNode := NewDeleteNodeStep()
	createTargetPool := NewCreateTargetPoolStep()
	createIPAddress := NewCreateAddressStep()
	createForwardingRules := NewCreateForwardingRulesStep()
	deleteForwardingRules := NewDeleteForwardingRulesStep()
	deleteTargetPool := NewDeleteTargetPoolStep()
	deleteIpAddress := NewDeleteIpAddressStep()

	steps.RegisterStep(CreateInstanceStepName, createInstance)
	steps.RegisterStep(DeleteClusterStepName, deleteCluster)
	steps.RegisterStep(DeleteNodeStepName, deleteNode)
	steps.RegisterStep(CreateTargetPullStepName, createTargetPool)
	steps.RegisterStep(CreateIPAddressStepName, createIPAddress)
	steps.RegisterStep(CreateForwardingRulesStepName, createForwardingRules)
	steps.RegisterStep(DeleteForwardingRulesStepName, deleteForwardingRules)
	steps.RegisterStep(DeleteTargetPoolStepName, deleteTargetPool)
	steps.RegisterStep(DeleteIpAddressStepName, deleteIpAddress)
}

func GetClient(ctx context.Context, config steps.GCEConfig) (*compute.Service, error) {
	data, err := json.Marshal(&config.ServiceAccount)

	if err != nil {
		return nil, errors.Wrapf(err, "Error marshalling service account")
	}

	opts := option.WithCredentialsJSON(data)

	computeService, err := compute.NewService(ctx, opts)

	if err != nil {
		return nil, err
	}
	return computeService, nil
}
