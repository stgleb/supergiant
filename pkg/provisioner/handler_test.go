package provisioner

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pkg/errors"

	"github.com/supergiant/supergiant/pkg/clouds"
	"github.com/supergiant/supergiant/pkg/model"
	"github.com/supergiant/supergiant/pkg/profile"
	"github.com/supergiant/supergiant/pkg/sgerrors"
	"github.com/supergiant/supergiant/pkg/workflows"
	"github.com/supergiant/supergiant/pkg/workflows/steps"
)

type mockTokenGetter struct {
	getToken func(context.Context, int) (string, error)
}

func (t *mockTokenGetter) GetToken(ctx context.Context, num int) (string, error) {
	return t.getToken(ctx, num)
}

type mockProvisioner struct {
	provision func(context.Context, *profile.Profile, *steps.Config) ([]*workflows.Task, error)
}

func (m *mockProvisioner) Provision(ctx context.Context, kubeProfile *profile.Profile, config *steps.Config) ([]*workflows.Task, error) {
	return m.provision(ctx, kubeProfile, config)
}

type mockAccountGetter struct {
	get func(context.Context, string) (*model.CloudAccount, error)
}

func (m *mockAccountGetter) Get(ctx context.Context, id string) (*model.CloudAccount, error) {
	return m.get(ctx, id)
}

func TestProvisionHandler(t *testing.T) {
	p := &ProvisionRequest{
		"test",
		profile.Profile{},
		"1234",
	}

	validBody, _ := json.Marshal(p)

	testCases := []struct {
		description string

		expectedCode int

		body       []byte
		getProfile func(context.Context, string) (*profile.Profile, error)
		getAccount func(context.Context, string) (*model.CloudAccount, error)
		getToken   func(context.Context, int) (string, error)
		provision  func(context.Context, *profile.Profile, *steps.Config) ([]*workflows.Task, error)
	}{
		{
			description:  "malformed request body",
			body:         []byte(`{`),
			expectedCode: http.StatusBadRequest,
		},
		{
			description:  "error getting the cluster discovery url",
			body:         validBody,
			expectedCode: http.StatusInternalServerError,
			getToken: func(context.Context, int) (string, error) {
				return "", errors.New("something has happened")
			},
		},
		{
			description:  "wrong cloud provider name",
			body:         validBody,
			expectedCode: http.StatusNotFound,
			getAccount: func(context.Context, string) (*model.CloudAccount, error) {
				return &model.CloudAccount{}, nil
			},
			getToken: func(context.Context, int) (string, error) {
				return "foo", nil
			},
		},
		{
			description:  "invalid credentials when provision",
			body:         validBody,
			expectedCode: http.StatusInternalServerError,
			getAccount: func(context.Context, string) (*model.CloudAccount, error) {
				return &model.CloudAccount{
					Provider: clouds.DigitalOcean,
				}, nil
			},
			getToken: func(context.Context, int) (string, error) {
				return "foo", nil
			},
			provision: func(context.Context, *profile.Profile, *steps.Config) ([]*workflows.Task, error) {
				return nil, sgerrors.ErrInvalidCredentials
			},
		},
		{
			body:         validBody,
			expectedCode: http.StatusAccepted,
			getAccount: func(context.Context, string) (*model.CloudAccount, error) {
				return &model.CloudAccount{
					Provider: clouds.DigitalOcean,
				}, nil
			},
			getToken: func(context.Context, int) (string, error) {
				return "foo", nil
			},
			provision: func(context.Context, *profile.Profile, *steps.Config) ([]*workflows.Task, error) {
				return []*workflows.Task{
					{
						ID: "master-task-id-1",
					},
					{
						ID: "node-task-id-2",
					},
				}, nil
			},
		},
	}

	provisioner := &mockProvisioner{}
	accGetter := &mockAccountGetter{}
	tokenGetter := &mockTokenGetter{}

	for _, testCase := range testCases {
		provisioner.provision = testCase.provision
		accGetter.get = testCase.getAccount
		tokenGetter.getToken = testCase.getToken

		req, _ := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(testCase.body))
		rec := httptest.NewRecorder()

		handler := Handler{
			provisioner:   provisioner,
			tokenGetter:   tokenGetter,
			accountGetter: accGetter,
		}

		handler.Provision(rec, req)

		if rec.Code != testCase.expectedCode {
			t.Errorf("Wrong status code expected %d actual %d", testCase.expectedCode, rec.Code)
			return
		}

		if testCase.expectedCode == http.StatusAccepted {
			resp := ProvisionResponse{}

			err := json.NewDecoder(rec.Body).Decode(&resp)

			if err != nil {
				t.Errorf("Unepxpected error while decoding response %v", err)
			}
		}
	}
}