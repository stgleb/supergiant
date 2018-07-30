package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/pborman/uuid"

	"github.com/supergiant/supergiant/pkg/storage"
	"github.com/supergiant/supergiant/pkg/workflows/steps"
)

type StepStatus struct {
	Status   steps.Status `json:"status"`
	StepName string       `json:"step_name"`
	ErrMsg   string       `json:"error_message"`
}

type WorkFlow struct {
	Type         string       `json:"type"`
	Config       steps.Config `json:"config"`
	StepStatuses []StepStatus `json:"steps"`

	workflowSteps []steps.Step
	repository    storage.Interface
}

const (
	MasterWorkFlow = "master"
	NodeWorkflow   = "node"
)

var (
	masterSteps = []steps.Step{}
	nodeSteps   = []steps.Step{}

	ErrUnknownWorkflowType = errors.New("unknown workflow type")
)

func New(workflowType string, config steps.Config, repository storage.Interface) (*WorkFlow, error) {
	if workflowType == MasterWorkFlow {
		return &WorkFlow{
			Config: config,

			workflowSteps: masterSteps,
			repository:    repository,
		}, nil
	} else if workflowType == NodeWorkflow {
		return &WorkFlow{
			Config: config,

			workflowSteps: nodeSteps,
			repository:    repository,
		}, nil
	}

	return nil, ErrUnknownWorkflowType
}

func (w *WorkFlow) Run(ctx context.Context, out io.Writer) (string, chan error) {
	errChan := make(chan error)
	id := uuid.New()

	go func() {
		// Create list of statuses to track
		for _, step := range w.workflowSteps {
			w.StepStatuses = append(w.StepStatuses, StepStatus{
				Status:   steps.StatusTodo,
				StepName: step.Name(),
				ErrMsg:   "",
			})
		}
		// Start from the first step
		w.startFrom(ctx, id, out, 0, errChan)
		close(errChan)
	}()

	return id, errChan
}

func (w *WorkFlow) Restart(ctx context.Context, id string, out io.Writer) chan error {
	errChan := make(chan error)

	go func() {
		defer close(errChan)
		data, err := w.repository.Get(ctx, "workflows", id)

		if err != nil {
			errChan <- err
			return
		}

		err = json.Unmarshal(data, w)

		if err != nil {
			errChan <- err
			return
		}

		i := 0
		// Skip successfully finished steps
		for index, stepStatus := range w.StepStatuses {
			if stepStatus.Status == steps.StatusError {
				i = index
				break
			}
		}
		// Start from the last failed one
		w.startFrom(ctx, id, out, i, errChan)
	}()
	return errChan
}

func (w *WorkFlow) startFrom(ctx context.Context, id string, out io.Writer, i int, errChan chan error) {
	// Start workflow from the last failed step
	for index := i; index < len(w.StepStatuses); index++ {
		step := w.workflowSteps[index]
		if err := step.Run(ctx, out, w.Config); err != nil {
			// Mark step status as error
			w.StepStatuses[index].Status = steps.StatusError
			w.StepStatuses[index].ErrMsg = err.Error()
			w.sync(ctx, id)

			errChan <- err
		} else {
			// Mark step as success
			w.StepStatuses[index].Status = steps.StatusSuccess
			w.sync(ctx, id)
		}
	}
}

func (w *WorkFlow) sync(ctx context.Context, id string) error {
	data, err := json.Marshal(w)

	if err != nil {
		return err
	}

	return w.repository.Put(ctx, "workflows", id, data)
}
