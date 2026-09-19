package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLocalProvider_DefaultConfig(t *testing.T) {
	provider := NewLocalProvider(OpenAIProviderConfig{})

	assert.Equal(t, "http://localhost:11434/v1", provider.baseURL)
	assert.Equal(t, "llama3", provider.model)
	assert.True(t, provider.skipAuth)
}

func TestLocalProvider_CustomConfig(t *testing.T) {
	provider := NewLocalProvider(OpenAIProviderConfig{
		BaseURL: "http://custom:11434/v1",
		Model:   "mistral",
	})

	assert.Equal(t, "http://custom:11434/v1", provider.baseURL)
	assert.Equal(t, "mistral", provider.model)
	assert.True(t, provider.skipAuth)
}

func TestOpenAIProvider_ProviderInfo(t *testing.T) {
	provider := NewOpenAIProvider(OpenAIProviderConfig{
		APIKey:  "test-key",
		BaseURL: "https://api.openai.com/v1",
		Model:   "gpt-4o",
	})

	info := provider.ProviderInfo()

	assert.Equal(t, "gpt-4o", info.Name)
	assert.Equal(t, "1.0", info.Version)
	assert.Equal(t, "openai", info.Provider)
}

func TestNoopProvider_ProviderInfo(t *testing.T) {
	provider := NewNoopProvider()

	info := provider.ProviderInfo()

	assert.Equal(t, "noop", info.Name)
	assert.Equal(t, "0.0", info.Version)
	assert.Equal(t, "none", info.Provider)
}
