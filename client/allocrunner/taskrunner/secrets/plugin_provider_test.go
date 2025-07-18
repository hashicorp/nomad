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

func (m *MockSecretPlugin) Fetch(ctx context.Context, path string) (*commonplugins.SecretResponse, error) {
	args := m.Called()

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*commonplugins.SecretResponse), args.Error(1)
}

func (m *MockSecretPlugin) Parse() (map[string]string, error) {
	return nil, nil
}

// SecretsPlugin is tested in commonplugins package. We can use a mock here to test how
// the ExternalPluginProvider handles various error scenarios when calling Fetch.
func TestExternalPluginProvider_Fetch(t *testing.T) {
	t.Run("errors if fetch errors", func(t *testing.T) {
		mockSecretPlugin := new(MockSecretPlugin)
		mockSecretPlugin.On("Fetch", mock.Anything).Return(nil, errors.New("something bad"))

		testProvider := NewExternalPluginProvider(mockSecretPlugin, "test", "test")

		err := testProvider.Fetch(context.Background())
		must.ErrorContains(t, err, "something bad")
	})

	t.Run("errors if fetch response contains error", func(t *testing.T) {
		mockSecretPlugin := new(MockSecretPlugin)
		testError := "something bad"
		mockSecretPlugin.On("Fetch", mock.Anything).Return(&commonplugins.SecretResponse{
			Result: nil,
			Error:  &testError,
		}, nil)

		testProvider := NewExternalPluginProvider(mockSecretPlugin, "test", "test")

		err := testProvider.Fetch(context.Background())
		must.ErrorContains(t, err, "error returned from secret plugin")
	})
}

func TestExternalPluginProvider_Parse(t *testing.T) {
	t.Run("formats response correctly", func(t *testing.T) {
		testProvider := NewExternalPluginProvider(nil, "test", "test")
		testProvider.response = &commonplugins.SecretResponse{
			Result: map[string]string{
				"testkey": "testvalue",
			},
		}

		result, err := testProvider.Parse()
		must.NoError(t, err)

		exp := map[string]string{
			"secret.test.testkey": "testvalue",
		}
		must.Eq(t, exp, result)

	})
	t.Run("errors if response is nil", func(t *testing.T) {
		testProvider := NewExternalPluginProvider(nil, "test", "test")
		testProvider.response = nil

		result, err := testProvider.Parse()
		must.Error(t, err)
		must.Nil(t, result)
	})
}
