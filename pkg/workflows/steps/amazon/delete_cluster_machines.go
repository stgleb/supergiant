package amazon

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/supergiant/control/pkg/clouds"
	"github.com/supergiant/control/pkg/util"
	"github.com/supergiant/control/pkg/workflows/steps"
)

const DeleteClusterMachinesStepName = "aws_delete_cluster_machines"

type DeleteClusterMachines struct {
	GetEC2 GetEC2Fn
}

func InitDeleteClusterMachines(fn GetEC2Fn) {
	steps.RegisterStep(DeleteClusterMachinesStepName, &DeleteClusterMachines{
		GetEC2: fn,
	})
}

func (s *DeleteClusterMachines) Run(ctx context.Context, w io.Writer, cfg *steps.Config) error {
	log := util.GetLogger(w)
	logrus.Infof("[%s] - deleting cluster %s machines",
		s.Name(), cfg.ClusterName)

	EC2, err := s.GetEC2(cfg.AWSConfig)
	if err != nil {
		return errors.Wrap(ErrAuthorization, err.Error())
	}

	logrus.Debug(cfg.AWSConfig)
	describeInstanceOutput, err := EC2.DescribeInstancesWithContext(ctx, &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String(fmt.Sprintf("tag:%s", clouds.ClusterIDTag)),
				Values: aws.StringSlice([]string{cfg.ClusterID}),
			},
		},
	})
	if err != nil {
		return errors.Wrap(ErrDeleteCluster, err.Error())
	}

	instanceIDS := make([]string, 0)
	for _, res := range describeInstanceOutput.Reservations {
		for _, instance := range res.Instances {
			instanceIDS = append(instanceIDS, *instance.InstanceId)
		}
	}
	if len(instanceIDS) == 0 {
		logrus.Infof("[%s] - no nodes in k8s cluster %s", s.Name(), cfg.ClusterName)
		return nil
	}

	output, err := EC2.TerminateInstancesWithContext(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: aws.StringSlice(instanceIDS),
	})

	if err != nil {
		logrus.Error(ErrDeleteCluster, err.Error())
		return errors.Wrap(ErrDeleteCluster, err.Error())
	}

	terminatedInstances := make([]string, 0)
	for _, instance := range output.TerminatingInstances {
		terminatedInstances = append(terminatedInstances, *instance.InstanceId)
	}

	msg := fmt.Sprintf("terminated instances %s", strings.Join(terminatedInstances, " , "))
	log.Infof("[%s] - %s", s.Name(), msg)
	logrus.Infof("[%s] Deleted AWS cluster %s, %s", s.Name(), cfg.ClusterName, msg)

	return nil
}

func (*DeleteClusterMachines) Name() string {
	return DeleteClusterMachinesStepName
}

func (*DeleteClusterMachines) Depends() []string {
	return nil
}

func (*DeleteClusterMachines) Description() string {
	return "Deletes all nodes in aws cluster"
}

func (*DeleteClusterMachines) Rollback(context.Context, io.Writer, *steps.Config) error {
	return nil
}
