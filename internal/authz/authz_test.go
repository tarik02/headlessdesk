package authz

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthorizeRequestRequiresAuthWhenAnyTokenConfigured(t *testing.T) {
	authorizer, err := New([]Token{
		{Value: "mcp-token", Audience: []string{"mcp"}, Scopes: []string{string(ScopeReadAll)}},
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/screenshot", nil)
	if got := authorizer.AuthorizeRequest(req, AudienceHTTP, ScopeReadScreenshot); got == nil || got.StatusCode != http.StatusUnauthorized {
		t.Fatalf("AuthorizeRequest() = %#v, want unauthorized even when token is for another audience", got)
	}

	req.Header.Set("Authorization", "Bearer mcp-token")
	if got := authorizer.AuthorizeRequest(req, AudienceHTTP, ScopeReadScreenshot); got == nil || got.StatusCode != http.StatusUnauthorized {
		t.Fatalf("AuthorizeRequest() = %#v, want unauthorized when token audience does not match", got)
	}

	req.Header.Del("Authorization")
	if got := authorizer.AuthorizeRequest(req, AudienceMCP, ScopeReadScreenshot); got == nil || got.StatusCode != http.StatusUnauthorized {
		t.Fatalf("AuthorizeRequest() = %#v, want unauthorized", got)
	}
}

func TestAuthorizeRequestMultipleTokensAndScopes(t *testing.T) {
	authorizer, err := New([]Token{
		{Value: "read-http", Audience: []string{"http"}, Scopes: []string{string(ScopeReadAll)}},
		{Value: "mouse-both", Audience: []string{"http", "mcp"}, Scopes: []string{string(ScopeWriteMouse)}},
		{Value: "admin", Scopes: []string{string(ScopeAll)}},
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name          string
		token         string
		audience      Audience
		scope         Scope
		wantStatus    int
		wantNoAuthErr bool
	}{
		{
			name:          "read wildcard allows screenshot",
			token:         "read-http",
			audience:      AudienceHTTP,
			scope:         ScopeReadScreenshot,
			wantNoAuthErr: true,
		},
		{
			name:       "read token cannot write mouse",
			token:      "read-http",
			audience:   AudienceHTTP,
			scope:      ScopeWriteMouse,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "http token cannot authenticate to mcp",
			token:      "read-http",
			audience:   AudienceMCP,
			scope:      ScopeReadScreenshot,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:          "both audience token works for mcp",
			token:         "mouse-both",
			audience:      AudienceMCP,
			scope:         ScopeWriteMouse,
			wantNoAuthErr: true,
		},
		{
			name:          "global wildcard allows any scope",
			token:         "admin",
			audience:      AudienceMCP,
			scope:         ScopeWriteKeyboard,
			wantNoAuthErr: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			req.Header.Set("Authorization", "Bearer "+tt.token)

			got := authorizer.AuthorizeRequest(req, tt.audience, tt.scope)
			if tt.wantNoAuthErr {
				if got != nil {
					t.Fatalf("AuthorizeRequest() = %v, want nil", got)
				}
				return
			}
			if got == nil || got.StatusCode != tt.wantStatus {
				t.Fatalf("AuthorizeRequest() = %#v, want status %d", got, tt.wantStatus)
			}
		})
	}
}

func TestAuthorizeScopes(t *testing.T) {
	authorizer, err := New([]Token{
		{Value: "token", Audience: []string{"mcp"}, Scopes: []string{string(ScopeReadAll)}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := authorizer.AuthorizeScopes(ScopeReadScreenshot, []string{string(ScopeReadAll)}); got != nil {
		t.Fatalf("AuthorizeScopes() = %v, want nil", got)
	}
	if got := authorizer.AuthorizeScopes(ScopeWriteMouse, []string{string(ScopeReadAll)}); got == nil || got.StatusCode != http.StatusForbidden {
		t.Fatalf("AuthorizeScopes() = %#v, want forbidden", got)
	}
	if got := authorizer.AuthorizeScopes(ScopeWriteMouse, nil); got == nil || got.StatusCode != http.StatusForbidden {
		t.Fatalf("AuthorizeScopes() = %#v, want forbidden when any token is configured", got)
	}
}

func TestAuthorizeRequestOpenWhenNoTokensConfigured(t *testing.T) {
	authorizer, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/screenshot", nil)
	if got := authorizer.AuthorizeRequest(req, AudienceHTTP, ScopeReadScreenshot); got != nil {
		t.Fatalf("AuthorizeRequest() = %v, want nil", got)
	}
}

func TestInvalidConfig(t *testing.T) {
	for _, tt := range []struct {
		name  string
		token Token
	}{
		{name: "empty token", token: Token{Scopes: []string{string(ScopeAll)}}},
		{name: "bad audience", token: Token{Value: "secret", Audience: []string{"ssh"}, Scopes: []string{string(ScopeAll)}}},
		{name: "bad scope", token: Token{Value: "secret", Scopes: []string{"read screenshot"}}},
		{name: "bad scope verb", token: Token{Value: "secret", Scopes: []string{"admin:*"}}},
		{name: "bad scope shape", token: Token{Value: "secret", Scopes: []string{"read:one:two"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New([]Token{tt.token}); err == nil {
				t.Fatal("New() error = nil, want error")
			}
		})
	}
}
