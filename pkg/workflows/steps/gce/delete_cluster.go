package gce

import (
	"context"
	"io"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	compute "google.golang.org/api/compute/v1"

	"github.com/supergiant/control/pkg/workflows/steps"
)

const DeleteClusterStepName = "gce_delete_cluster"

type DeleteClusterStep struct {
	// Client creates the client for the provider.
	getClient func(context.Context, string, string, string) (*compute.Service, error)
}

func NewDeleteClusterStep() (steps.Step, error) {
	return &DeleteClusterStep{
		getClient: GetClient,
	}, nil
}

func (s *DeleteClusterStep) Run(ctx context.Context, output io.Writer, config *steps.Config) error {
	// fetch client.
	client, err := s.getClient(ctx, config.GCEConfig.ClientEmail,
		config.GCEConfig.PrivateKey, config.GCEConfig.TokenURI)
	if err != nil {
		return err
	}

	for _, master := range config.GetMasters() {
		logrus.Debugf("Delete master %s in %s", master.Name, master.Region)

		_, serr := client.Instances.Delete(config.GCEConfig.ProjectID,
			master.Region,
			master.Name).Do()

		if serr != nil {
			return errors.Wrap(serr, "GCE delete instance")
		}
	}

	for _, node := range config.GetNodes() {
		logrus.Debugf("Delete node %s in %s", node.Name, node.Region)
		_, serr := client.Instances.Delete(config.GCEConfig.ProjectID,
			node.Region,
			node.Name).Do()

		if serr != nil {
			return errors.Wrap(serr, "GCE delete instance")
		}
	}

	return nil
}

func (s *DeleteClusterStep) Name() string {
	return DeleteClusterStepName
}

func (s *DeleteClusterStep) Depends() []string {
	return nil
}

func (s *DeleteClusterStep) Description() string {
	return "Google compute engine step for creating instance"
}

func (s *DeleteClusterStep) Rollback(context.Context, io.Writer, *steps.Config) error {
	return nil
}
