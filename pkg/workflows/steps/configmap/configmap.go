package configmap

import (
	"context"
	"encoding/json"
	"github.com/supergiant/control/pkg/clouds"
	"github.com/supergiant/control/pkg/util"
	"io"
	"time"

	"github.com/pkg/errors"

	"github.com/sirupsen/logrus"

	"k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/supergiant/control/pkg/kubeconfig"
	"github.com/supergiant/control/pkg/model"
	"github.com/supergiant/control/pkg/workflows/steps"
)

const StepName = "configmap"

type Step struct {
	timeout      time.Duration
	attemptCount int
}

func New() *Step {
	t := &Step{
		timeout:      time.Minute * 1,
		attemptCount: 5,
	}

	return t
}

func Init() {
	steps.RegisterStep(StepName, New())
}

func (s *Step) Run(ctx context.Context, out io.Writer, config *steps.Config) error {
	k8sClient, err := s.getK8SClient(config)

	if err != nil {
		return errors.Wrap(err, "get k8s client")
	}

	timeout := s.timeout

	for i := 0; i < s.attemptCount; i++ {
		_, err = k8sClient.Namespaces().Create(&v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: clouds.CapacityNamespace,
			},
		})

		if k8serrors.IsAlreadyExists(err) || err == nil {
			break
		}

		logrus.Debugf("create namespace error %v", err)
		time.Sleep(s.timeout)
		timeout *= 2
	}

	if !k8serrors.IsAlreadyExists(err) && err != nil {
		return errors.Wrap(err, "create namespace for capacity configmap")
	}

	util.UpdateKubeWithCloudSpecificData(&config.Kube, config)

	var data []byte
	if data, err = json.Marshal(config.Kube.CloudSpec); err != nil {
		return errors.Wrap(err, "marshalling cloud specific map")
	}

	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: clouds.CapacityNamespace,
			Name:      clouds.CapacityProvisionConfigMap,
		},
		Data: map[string]string{
			clouds.CapacityScriptKey:        config.ConfigMap.Data,
			clouds.CapacityCloudSpecificKey: string(data),
		},
	}

	_, err = k8sClient.ConfigMaps(clouds.CapacityNamespace).Create(configMap)

	if err != nil {
		return errors.Wrapf(err, "create config map")
	}

	return nil
}

func (s *Step) Rollback(ctx context.Context, out io.Writer, config *steps.Config) error {
	k8sClient, err := s.getK8SClient(config)

	if err != nil {
		return errors.Wrap(err, "get k8s client")
	}

	err = k8sClient.Namespaces().Delete(clouds.CapacityNamespace, nil)

	if err != nil {
		return errors.Wrap(err, "delete capacity namespace")
	}

	return nil
}

func (s *Step) Name() string {
	return StepName
}

func (s *Step) Description() string {
	return "create configmap for capacity service"
}

func (s *Step) Depends() []string {
	return nil
}

func (s *Step) getK8SClient(config *steps.Config) (*clientcorev1.CoreV1Client, error) {
	var err error

	config.Kube.Auth.AdminCert = config.CertificatesConfig.AdminCert
	config.Kube.Auth.AdminKey = config.CertificatesConfig.AdminKey
	config.Kube.Auth.CACert = config.CertificatesConfig.CACert
	config.Kube.ExternalDNSName = config.ExternalDNSName

	master := config.GetMaster()
	config.Kube.Masters = map[string]*model.Machine{
		master.Name: master,
	}

	cfg, err := kubeconfig.NewConfigFor(&config.Kube)

	if err != nil {
		return nil, errors.Wrap(err, "configmap create kubeconfig from kube")
	}

	k8sClient, err := clientcorev1.NewForConfig(cfg)

	if err != nil {
		return nil, errors.Wrapf(err, "configmap build kubernetes client")
	}

	return k8sClient, nil
}
