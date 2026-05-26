package servercmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestLoadConfigAuthTokens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte(`
server:
  auth:
    tokens:
      - token: http-secret
        audience: [http]
        scopes: [read:screenshot]
      - token: both-secret
        scopes: ["*"]
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	v := viper.New()
	setDefaults(v)
	cfg, err := loadConfig(v, path)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(cfg.Server.Auth.Tokens); got != 2 {
		t.Fatalf("len(tokens) = %d, want 2", got)
	}
	if got := cfg.Server.Auth.Tokens[0].Audience; len(got) != 1 || got[0] != "http" {
		t.Fatalf("tokens[0].audience = %#v, want [http]", got)
	}
	if _, err := newAuthorizer(cfg.Server.Auth); err != nil {
		t.Fatalf("newAuthorizer() error = %v", err)
	}
}
