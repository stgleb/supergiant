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
	getComputeSvc func(context.Context, steps.GCEConfig) (*computeService, error)
}

func NewDeleteClusterStep() (*DeleteClusterStep, error) {
	return &DeleteClusterStep{
		getComputeSvc: func(ctx context.Context, config steps.GCEConfig) (*computeService, error) {
			client, err := GetClient(ctx, config.ClientEmail,
				config.PrivateKey, config.TokenURI)

			if err != nil {
				return nil, err
			}

			return &computeService{
				deleteInstance: func(projectID string, region string, name string) (*compute.Operation, error) {
					return client.Instances.Delete(projectID, region, name).Do()
				},
			}, nil
		},
	}, nil
}

func (s *DeleteClusterStep) Run(ctx context.Context, output io.Writer, config *steps.Config) error {
	// fetch client.
	svc, err := s.getComputeSvc(ctx, config.GCEConfig)

	if err != nil {
		return errors.Wrapf(err, "%s get service", DeleteClusterStepName)
	}

	for _, master := range config.GetMasters() {
		logrus.Debugf("Delete master %s in %s", master.Name, master.Region)

		_, serr := svc.deleteInstance(config.GCEConfig.ProjectID,
			master.Region,
			master.Name)

		if serr != nil {
			return errors.Wrap(serr, "GCE delete instance")
		}
	}

	for _, node := range config.GetNodes() {
		logrus.Debugf("Delete node %s in %s", node.Name, node.Region)
		_, serr := svc.deleteInstance(config.GCEConfig.ProjectID,
			node.Region,
			node.Name)

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
	return "Google compute engine delete cluster step"
}

func (s *DeleteClusterStep) Rollback(context.Context, io.Writer, *steps.Config) error {
	return nil
}
