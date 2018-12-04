package workflows

import (
	"encoding/json"

	"github.com/supergiant/control/pkg/storage"
	"github.com/supergiant/control/pkg/runner/ssh"
)

func DeserializeTask(data []byte, repository storage.Interface) (*Task, error) {
	task := &Task{}
	err := json.Unmarshal(data, task)

	if err != nil {
		return nil, err
	}

	// Assign repository from task handler to task and restore workflow
	task.repository = repository
	task.workflow = GetWorkflow(task.Type)


	// NOTE(stgleb): If step has failed on machine creation state
	// public ip will be blank and lead to error when restart
	// TODO(stgleb): Move ssh runner creation to task Restart method
	if task.Config.Node.PublicIp != "" {
		cfg := ssh.Config{
			Host:    task.Config.Node.PublicIp,
			Port:    task.Config.SshConfig.Port,
			User:    task.Config.SshConfig.User,
			Timeout: task.Config.SshConfig.Timeout,
			Key:     []byte(task.Config.SshConfig.BootstrapPrivateKey),
		}

		task.Config.Runner, err = ssh.NewRunner(cfg)

		if err != nil {
			return nil, err
		}
	}

	return task, nil
}
