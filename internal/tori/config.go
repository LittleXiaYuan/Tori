package tori

import (
	"os"
	"strings"
	"sync"
)

type OAuthConfig struct {
	ToriBaseURL         string
	ClientID            string
	CallbackPort        int
	Scopes              string
	CodeChallengeMethod string
}

const DefaultToriURL = "https://tori.owo.today"

func DefaultOAuthConfig() OAuthConfig {
	base := os.Getenv("TORI_URL")
	if base == "" {
		base = DefaultToriURL
	}
	return OAuthConfig{
		ToriBaseURL:         base,
		ClientID:            "yunque-agent",
		CallbackPort:        18921,
		Scopes:              "api",
		CodeChallengeMethod: "S256",
	}
}

var (
	savedLLM struct {
		baseURL        string
		apiKey         string
		sandboxBaseURL string
		sandboxAPIKey  string
	}
	savedOnce sync.Once
)

func ApplyLLMConfig(toriURL, apiKey string) {
	savedOnce.Do(func() {
		savedLLM.baseURL = os.Getenv("LLM_BASE_URL")
		savedLLM.apiKey = os.Getenv("LLM_API_KEY")
		savedLLM.sandboxBaseURL = os.Getenv("SANDBOX_CLOUD_BASE_URL")
		savedLLM.sandboxAPIKey = os.Getenv("SANDBOX_CLOUD_API_KEY")
	})
	os.Setenv("LLM_BASE_URL", toriURL)
	if apiKey != "" {
		os.Setenv("LLM_API_KEY", apiKey)
		if os.Getenv("SANDBOX_CLOUD_API_KEY") == "" {
			os.Setenv("SANDBOX_CLOUD_API_KEY", apiKey)
			os.Setenv("SANDBOX_CLOUD_BASE_URL", strings.TrimRight(toriURL, "/")+"/v1")
		}
	}
}

func RestoreLLMConfig() {
	os.Setenv("LLM_BASE_URL", savedLLM.baseURL)
	os.Setenv("LLM_API_KEY", savedLLM.apiKey)
	os.Setenv("SANDBOX_CLOUD_BASE_URL", savedLLM.sandboxBaseURL)
	os.Setenv("SANDBOX_CLOUD_API_KEY", savedLLM.sandboxAPIKey)
	savedOnce = sync.Once{}
}
