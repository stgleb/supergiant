package openstack

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"text/template"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/images"

	tm "github.com/supergiant/control/pkg/templatemanager"
	"github.com/supergiant/control/pkg/workflows/steps"
	"github.com/supergiant/control/pkg/workflows/steps/kubeadm"
)

const FindImageStepName = "find_image"

type FindImageStep struct {
	script    *template.Template
	getClient func(steps.OpenStackConfig) (*gophercloud.ProviderClient, error)
}

func Init() {
	tpl, err := tm.GetTemplate(FindImageStepName)

	if err != nil {
		panic(fmt.Sprintf("template %s not found", FindImageStepName))
	}

	steps.RegisterStep(FindImageStepName, New(tpl))
}

func New(tpl *template.Template) *FindImageStep {
	return &FindImageStep{
		script: tpl,
		getClient: func(config steps.OpenStackConfig) (client *gophercloud.ProviderClient, e error) {
			opts := gophercloud.AuthOptions{
				IdentityEndpoint: config.AuthURL,
				Username:         config.UserName,
				Password:         config.Password,
				TenantID:         config.TenantID,
				DomainID:         config.DomainID,
				DomainName:       config.DomainName,
			}

			client, err := openstack.AuthenticatedClient(opts)
			if err != nil {
				return nil, err
			}

			return client, nil
		},
	}
}

func (s *FindImageStep) Run(ctx context.Context, out io.Writer, config *steps.Config) error {
	client, err := s.getClient(config.OpenStackConfig)

	if err != nil {
		return errors.Wrapf(err, "step %s", FindImageStepName)
	}

	computeClient, err := openstack.NewComputeV2(client, gophercloud.EndpointOpts{
		Region: config.OpenStackConfig.Region,
	})

	if err != nil {
		return errors.Wrapf(err, "step %s get compute client", FindImageStepName)
	}

	ImageID, err := images.IDFromName(computeClient, "ubuntu")

	if err != nil {
		return errors.Wrapf(err, "step %s get image id", FindImageStepName)
	}

	config.OpenStackConfig.ImageID = ImageID

	return nil
}

func (s *FindImageStep) Name() string {
	return FindImageStepName
}

func (s *FindImageStep) Rollback(context.Context, io.Writer, *steps.Config) error {
	return nil
}

func (s *FindImageStep) Description() string {
	return "Find appropriate image id in glance service"
}

func (s *FindImageStep) Depends() []string {
	return []string{kubeadm.StepName}
}