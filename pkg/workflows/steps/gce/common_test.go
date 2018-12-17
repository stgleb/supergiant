package gce

import (
	"context"
	"github.com/supergiant/control/pkg/workflows/steps"
	"testing"
)

func TestNewCreateInstanceStep2(t *testing.T) {
	Init()

	createStep := steps.GetStep(CreateInstanceStepName)

	if createStep == nil {
		t.Errorf("Create instance step must not be nil")
	}

	deleteClusterStep := steps.GetStep(DeleteNodeStepName)

	if deleteClusterStep == nil {
		t.Errorf("Delete cluster step must not be nil")
	}

	deleteNode := steps.GetStep(DeleteNodeStepName)

	if deleteNode == nil {
		t.Errorf("Delete node must not be nil")
	}
}

func TestGetClient(t *testing.T) {
	client, err := GetClient(context.Background(), "", "", "")

	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}

	if client == nil {
		t.Errorf("Client must not be nil")
	}
}
