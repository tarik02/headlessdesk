package authz

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthorizeRequestOptionalPerAudience(t *testing.T) {
	authorizer, err := New([]Token{
		{Value: "mcp-token", Audience: []string{"mcp"}, Scopes: []string{"read:*"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/screenshot", nil)
	if got := authorizer.AuthorizeRequest(req, AudienceHTTP, "read:screenshot"); got != nil {
		t.Fatalf("AuthorizeRequest() = %v, want nil when audience has no tokens", got)
	}

	if got := authorizer.AuthorizeRequest(req, AudienceMCP, "read:screenshot"); got == nil || got.StatusCode != http.StatusUnauthorized {
		t.Fatalf("AuthorizeRequest() = %#v, want unauthorized for configured audience", got)
	}
}

func TestAuthorizeRequestMultipleTokensAndScopes(t *testing.T) {
	authorizer, err := New([]Token{
		{Value: "read-http", Audience: []string{"http"}, Scopes: []string{"read:*"}},
		{Value: "mouse-both", Audience: []string{"http", "mcp"}, Scopes: []string{"write:mouse"}},
		{Value: "admin", Scopes: []string{"*"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name          string
		token         string
		audience      Audience
		scope         string
		wantStatus    int
		wantNoAuthErr bool
	}{
		{
			name:          "read wildcard allows screenshot",
			token:         "read-http",
			audience:      AudienceHTTP,
			scope:         "read:screenshot",
			wantNoAuthErr: true,
		},
		{
			name:       "read token cannot write mouse",
			token:      "read-http",
			audience:   AudienceHTTP,
			scope:      "write:mouse",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "http token cannot authenticate to mcp",
			token:      "read-http",
			audience:   AudienceMCP,
			scope:      "read:screenshot",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:          "both audience token works for mcp",
			token:         "mouse-both",
			audience:      AudienceMCP,
			scope:         "write:mouse",
			wantNoAuthErr: true,
		},
		{
			name:          "global wildcard allows any scope",
			token:         "admin",
			audience:      AudienceMCP,
			scope:         "write:keyboard",
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
		{Value: "token", Audience: []string{"mcp"}, Scopes: []string{"read:*"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := authorizer.AuthorizeScopes(AudienceMCP, "read:screenshot", []string{"read:*"}); got != nil {
		t.Fatalf("AuthorizeScopes() = %v, want nil", got)
	}
	if got := authorizer.AuthorizeScopes(AudienceMCP, "write:mouse", []string{"read:*"}); got == nil || got.StatusCode != http.StatusForbidden {
		t.Fatalf("AuthorizeScopes() = %#v, want forbidden", got)
	}
	if got := authorizer.AuthorizeScopes(AudienceHTTP, "write:mouse", nil); got != nil {
		t.Fatalf("AuthorizeScopes() = %v, want nil when audience has no tokens", got)
	}
}

func TestInvalidConfig(t *testing.T) {
	for _, tt := range []struct {
		name  string
		token Token
	}{
		{name: "empty token", token: Token{Scopes: []string{"*"}}},
		{name: "bad audience", token: Token{Value: "secret", Audience: []string{"ssh"}, Scopes: []string{"*"}}},
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
