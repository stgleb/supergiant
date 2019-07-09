package authorizedkeys

import (
	"context"
	"fmt"
	"github.com/supergiant/control/pkg/clouds"
	"io"
	"text/template"

	"github.com/pkg/errors"

	"github.com/supergiant/control/pkg/templatemanager"
	"github.com/supergiant/control/pkg/util"
	"github.com/supergiant/control/pkg/workflows/steps"
)

type Step struct {
	script *template.Template
}

const StepName = "add_authorized_keys"

func Init() {
	tpl, err := templatemanager.GetTemplate(StepName)

	if err != nil {
		panic(fmt.Sprintf("template %s not found", StepName))
	}
	steps.RegisterStep(StepName, NewAddAuthorizedKeys(tpl))
}

func NewAddAuthorizedKeys(script *template.Template) *Step {
	return &Step{
		script: script,
	}
}

func (s *Step) Run(ctx context.Context, w io.Writer, cfg *steps.Config) error {
	log := util.GetLogger(w)

	log.Infof("[%s] - adding user's public key to the node", s.Name())
	if cfg == nil || cfg.Kube.SSHConfig.PublicKey != "" {
		err := steps.RunTemplate(ctx, s.script, cfg.Runner, w, struct {
			PublicKey          string
			BootstrapPublicKey string
			UserName           string
		}{
			PublicKey:          cfg.Kube.SSHConfig.PublicKey,
			BootstrapPublicKey: cfg.Kube.SSHConfig.BootstrapPublicKey,
			UserName:           clouds.OSUser,
		})
		if err != nil {
			return errors.Wrap(err, "add authorized key step")
		}
	} else {
		log.Infof("[%s] - no public key provided, skipping...", s.Name())
	}

	// Use this user next steps
	cfg.Kube.SSHConfig.User = clouds.OSUser

	return nil
}

func (*Step) Name() string {
	return StepName
}

func (*Step) Description() string {
	return "adds ssh public key to the authorized keys file"
}

func (*Step) Depends() []string {
	return nil
}

func (*Step) Rollback(context.Context, io.Writer, *steps.Config) error {
	return nil
}
