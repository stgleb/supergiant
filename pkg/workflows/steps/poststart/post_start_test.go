package poststart

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/pkg/errors"

	"github.com/supergiant/control/pkg/clouds"
	"github.com/supergiant/control/pkg/model"
	"github.com/supergiant/control/pkg/profile"
	"github.com/supergiant/control/pkg/runner"
	"github.com/supergiant/control/pkg/templatemanager"
	"github.com/supergiant/control/pkg/workflows/steps"
	"github.com/supergiant/control/pkg/workflows/steps/kubelet"
)

type fakeRunner struct {
	errMsg  string
	timeout time.Duration
}

func (f *fakeRunner) Run(command *runner.Command) error {
	if len(f.errMsg) > 0 {
		return errors.New(f.errMsg)
	}

	_, err := io.Copy(command.Out, strings.NewReader(command.Script))
	// Simulate command latency
	time.Sleep(f.timeout)

	return err
}

func TestPostStartMaster(t *testing.T) {
	r := &fakeRunner{}

	err := templatemanager.Init("../../../../templates")

	if err != nil {
		t.Fatal(err)
	}

	tpl, _ := templatemanager.GetTemplate(StepName)

	if tpl == nil {
		t.Fatal("template not found")
	}

	output := new(bytes.Buffer)
	cfg, err := steps.NewConfig("test", "test", profile.Profile{
		MasterProfiles: []profile.NodeProfile{{}},
	})

	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}

	cfg.IsMaster = true
	cfg.PostStartConfig = steps.PostStartConfig{
		Timeout: time.Second * 10,
	}
	cfg.Node = model.Machine{
		PrivateIp: "10.20.30.40",
	}
	cfg.Runner = r

	j := &Step{
		tpl,
	}

	err = j.Run(context.Background(), output, cfg)

	if err != nil {
		t.Errorf("Unpexpected error while master node %v", err)
	}
}

func TestPostStartNode(t *testing.T) {
	r := &fakeRunner{}

	err := templatemanager.Init("../../../../templates")

	if err != nil {
		t.Fatal(err)
	}

	tpl, _ := templatemanager.GetTemplate(StepName)

	if tpl == nil {
		t.Fatal("template not found")
	}

	output := new(bytes.Buffer)
	cfg, err := steps.NewConfig("test", "test", profile.Profile{
		MasterProfiles: []profile.NodeProfile{{}},
	})

	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}

	cfg.PostStartConfig = steps.PostStartConfig{
		Timeout: time.Second * 10,
	}
	cfg.Node = model.Machine{
		PrivateIp: "10.20.30.40",
	}
	cfg.Runner = r

	j := &Step{
		tpl,
	}

	err = j.Run(context.Background(), output, cfg)

	if err != nil {
		t.Errorf("Unpexpected error while  provision node %v", err)
	}
}

func TestPostStartError(t *testing.T) {
	errMsg := "error has occurred"

	r := &fakeRunner{
		errMsg: errMsg,
	}

	proxyTemplate, err := template.New(StepName).Parse("")
	output := new(bytes.Buffer)

	j := &Step{
		proxyTemplate,
	}

	cfg := &steps.Config{
		PostStartConfig: steps.PostStartConfig{
			Timeout: time.Second * 10,
		},
		Runner: r,
	}
	err = j.Run(context.Background(), output, cfg)

	if err == nil {
		t.Errorf("Error must not be nil")
		return
	}

	if !strings.Contains(err.Error(), errMsg) {
		t.Errorf("Error message expected to contain %s actual %s", errMsg, err.Error())
	}
}

func TestPostStartTimeout(t *testing.T) {
	port := "8080"
	username := "john"
	rbacEnabled := true

	r := &fakeRunner{
		errMsg:  "",
		timeout: time.Second * 2,
	}

	proxyTemplate, err := template.New(StepName).Parse("")
	output := new(bytes.Buffer)

	j := &Step{
		proxyTemplate,
	}

	cfg, err := steps.NewConfig("", "", profile.Profile{})

	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}

	cfg.PostStartConfig = steps.PostStartConfig{
		IsMaster:    true,
		Provider:    clouds.AWS,
		Host:        "127.0.0.1",
		Port:        port,
		Username:    username,
		RBACEnabled: rbacEnabled,
		Timeout:     1,
	}
	cfg.Runner = r

	err = j.Run(context.Background(), output, cfg)

	if err == nil {
		t.Errorf("Error must not be nil")
		return
	}

	if errors.Cause(err) != context.DeadlineExceeded {
		t.Errorf("Wrong error cause expected %v actual %v", context.DeadlineExceeded,
			err)
	}
}

func TestStepName(t *testing.T) {
	s := Step{}

	if s.Name() != StepName {
		t.Errorf("Unexpected step name expected %s actual %s", StepName, s.Name())
	}
}

func TestDepends(t *testing.T) {
	s := Step{}

	if len(s.Depends()) != 1 && s.Depends()[0] != kubelet.StepName {
		t.Errorf("Wrong dependency list %v expected %v", s.Depends(), []string{kubelet.StepName})
	}
}

func TestStep_Rollback(t *testing.T) {
	s := Step{}
	err := s.Rollback(context.Background(), ioutil.Discard, &steps.Config{})

	if err != nil {
		t.Errorf("unexpected error while rollback %v", err)
	}
}

func TestNew(t *testing.T) {
	tpl := template.New("test")
	s := New(tpl)

	if s.script != tpl {
		t.Errorf("Wrong template expected %v actual %v", tpl, s.script)
	}
}

func TestInit(t *testing.T) {
	templatemanager.SetTemplate(StepName, &template.Template{})
	Init()
	templatemanager.DeleteTemplate(StepName)

	s := steps.GetStep(StepName)

	if s == nil {
		t.Error("Step not found")
	}
}

func TestInitPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("recover output must not be nil")
		}
	}()

	Init()

	s := steps.GetStep("not_found.sh.tpl")

	if s == nil {
		t.Error("Step not found")
	}
}

func TestStep_Description(t *testing.T) {
	s := &Step{}

	if desc := s.Description(); desc != "Post start step executes after "+
		"provisioning" {
		t.Errorf("Wrong desription expected %s actual %s",
			"Post start step executes after provisioning", desc)
	}
}
