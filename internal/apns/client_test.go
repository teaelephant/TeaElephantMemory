package apns

import (
	"testing"

	"github.com/sideshow/apns2/token"
)

func TestNewClientSelectsEnvironment(t *testing.T) {
	tests := []struct {
		environment string
		wantHost    string
	}{
		{environment: "sandbox", wantHost: "https://api.sandbox.push.apple.com"},
		{environment: "production", wantHost: "https://api.push.apple.com"},
	}

	for _, tt := range tests {
		t.Run(tt.environment, func(t *testing.T) {
			client, err := NewClient(&token.Token{}, tt.environment)
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}

			if client.Host != tt.wantHost {
				t.Errorf("NewClient() host = %q, want %q", client.Host, tt.wantHost)
			}
		})
	}
}

func TestNewClientRejectsUnknownEnvironment(t *testing.T) {
	for _, environment := range []string{"", "development", "prod", "Production", " production", "sandbox "} {
		t.Run(environment, func(t *testing.T) {
			client, err := NewClient(&token.Token{}, environment)
			if err == nil {
				t.Fatal("NewClient() succeeded for an unsupported environment")
			}

			if client != nil {
				t.Error("NewClient() returned a client for an unsupported environment")
			}
		})
	}
}
