// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package secrets

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/nomad/client/commonplugins"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/mock"
)

type MockSecretPlugin struct {
	mock.Mock
}

func (m *MockSecretPlugin) Fingerprint(ctx context.Context) (*commonplugins.PluginFingerprint, error) {
	return nil, nil
}

func (m *MockSecretPlugin) Fetch(ctx context.Context, path string, env map[string]string) (*commonplugins.SecretResponse, error) {
	args := m.Called(ctx, path, env)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*commonplugins.SecretResponse), args.Error(1)
}

func (m *MockSecretPlugin) Parse() (map[string]string, error) {
	return nil, nil
}

func (m *MockSecretPlugin) SetEnv(env map[string]string) {
	m.Called(env)
}

// SecretsPlugin is tested in commonplugins package. We can use a mock here to test how
// the ExternalPluginProvider handles various error scenarios when calling Fetch.
func TestExternalPluginProvider_Fetch(t *testing.T) {
	t.Run("errors if fetch errors", func(t *testing.T) {
		mockSecretPlugin := new(MockSecretPlugin)
		mockSecretPlugin.On("Fetch", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("something bad"))

		testProvider := NewExternalPluginProvider(mockSecretPlugin, "test-provider", "test", "test")

		vars, err := testProvider.Fetch(t.Context(), nil)
		must.ErrorContains(t, err, "something bad")
		must.Nil(t, vars)
	})

	t.Run("errors if fetch response contains error", func(t *testing.T) {
		mockSecretPlugin := new(MockSecretPlugin)
		testError := "something bad"
		mockSecretPlugin.On("Fetch", mock.Anything, mock.Anything, mock.Anything).Return(&commonplugins.SecretResponse{
			Result: nil,
			Error:  &testError,
		}, nil)

		testProvider := NewExternalPluginProvider(mockSecretPlugin, "test-provider", "test", "test")

		vars, err := testProvider.Fetch(t.Context(), nil)
		must.ErrorContains(t, err, "provider \"test-provider\" for secret \"test\" response contained error")
		must.Nil(t, vars)
	})

	t.Run("formats response correctly", func(t *testing.T) {
		mockSecretPlugin := new(MockSecretPlugin)
		mockSecretPlugin.On("Fetch", mock.Anything, mock.Anything, mock.Anything).Return(&commonplugins.SecretResponse{
			Result: map[string]string{
				"testkey": "testvalue",
			},
			Error: nil,
		}, nil)

		testProvider := NewExternalPluginProvider(mockSecretPlugin, "test-provider", "test", "test")

		result, err := testProvider.Fetch(t.Context(), nil)
		must.NoError(t, err)

		exp := map[string]string{
			"secret.test.testkey": "testvalue",
		}
		must.Eq(t, exp, result)
	})
}
