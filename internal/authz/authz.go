package authz

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

type Audience string

const (
	AudienceHTTP Audience = "http"
	AudienceMCP  Audience = "mcp"
)

type Scope string

const (
	ScopeAll            Scope = "*"
	ScopeReadAll        Scope = "read:*"
	ScopeReadStatus     Scope = "read:status"
	ScopeReadScreenshot Scope = "read:screenshot"
	ScopeReadWait       Scope = "read:wait"
	ScopeWriteAll       Scope = "write:*"
	ScopeWriteMouse     Scope = "write:mouse"
	ScopeWriteKeyboard  Scope = "write:keyboard"
)

type Token struct {
	Value     string
	Audience  []string
	Audiences []string
	Scopes    []string
}

type Authorizer struct {
	tokens []storedToken
}

type storedToken struct {
	value     string
	audiences map[Audience]bool
	scopes    []Scope
	userID    string
}

type Error struct {
	StatusCode int
	Message    string
	Scope      Scope
}

func (e *Error) Error() string {
	if e.Scope == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: requires %s", e.Message, e.Scope)
}

func New(tokens []Token) (*Authorizer, error) {
	a := &Authorizer{}
	for i, token := range tokens {
		stored, err := normalizeToken(token)
		if err != nil {
			return nil, fmt.Errorf("tokens[%d]: %w", i, err)
		}
		a.tokens = append(a.tokens, stored)
	}
	return a, nil
}

func (a *Authorizer) Configured() bool {
	if a == nil {
		return false
	}
	return len(a.tokens) > 0
}

func (a *Authorizer) AuthorizeRequest(req *http.Request, audience Audience, requiredScope Scope) *Error {
	if !a.Configured() {
		return nil
	}
	bearer, ok := bearerToken(req.Header.Get("Authorization"))
	if !ok {
		return &Error{StatusCode: http.StatusUnauthorized, Message: "missing bearer token", Scope: requiredScope}
	}
	token := a.findToken(audience, bearer)
	if token == nil {
		return &Error{StatusCode: http.StatusUnauthorized, Message: "invalid bearer token", Scope: requiredScope}
	}
	if !scopesAllow(token.scopes, requiredScope) {
		return &Error{StatusCode: http.StatusForbidden, Message: "insufficient scope", Scope: requiredScope}
	}
	return nil
}

func (a *Authorizer) AuthorizeScopes(requiredScope Scope, scopes []string) *Error {
	if !a.Configured() {
		return nil
	}
	if !scopesAllow(scopesFromStrings(scopes), requiredScope) {
		return &Error{StatusCode: http.StatusForbidden, Message: "insufficient scope", Scope: requiredScope}
	}
	return nil
}

func (a *Authorizer) MCPMiddleware(next http.Handler) http.Handler {
	if !a.Configured() {
		return next
	}
	verifier := func(_ context.Context, bearer string, _ *http.Request) (*mcpauth.TokenInfo, error) {
		token := a.findToken(AudienceMCP, bearer)
		if token == nil {
			return nil, mcpauth.ErrInvalidToken
		}
		return &mcpauth.TokenInfo{
			Scopes:     scopeStrings(token.scopes),
			Expiration: time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC),
			UserID:     token.userID,
		}, nil
	}
	return mcpauth.RequireBearerToken(verifier, nil)(next)
}

func (a *Authorizer) findToken(audience Audience, bearer string) *storedToken {
	for i := range a.tokens {
		token := &a.tokens[i]
		if token.hasAudience(audience) && constantTimeEqual(token.value, bearer) {
			return token
		}
	}
	return nil
}

func (t storedToken) hasAudience(audience Audience) bool {
	return t.audiences[audience]
}

func normalizeToken(token Token) (storedToken, error) {
	value := strings.TrimSpace(token.Value)
	if value == "" {
		return storedToken{}, errors.New("token is required")
	}

	audienceValues := append([]string(nil), token.Audience...)
	audienceValues = append(audienceValues, token.Audiences...)
	audiences, err := normalizeAudiences(audienceValues)
	if err != nil {
		return storedToken{}, err
	}

	scopes := make([]Scope, 0, len(token.Scopes))
	for _, scope := range token.Scopes {
		scope = strings.TrimSpace(scope)
		if !ValidScope(Scope(scope)) {
			return storedToken{}, fmt.Errorf("invalid scope %q", scope)
		}
		scopes = append(scopes, Scope(scope))
	}

	sum := sha256.Sum256([]byte(value))
	return storedToken{
		value:     value,
		audiences: audiences,
		scopes:    scopes,
		userID:    hex.EncodeToString(sum[:]),
	}, nil
}

func normalizeAudiences(values []string) (map[Audience]bool, error) {
	audiences := map[Audience]bool{}
	if len(values) == 0 {
		audiences[AudienceHTTP] = true
		audiences[AudienceMCP] = true
		return audiences, nil
	}
	for _, value := range values {
		switch audience := Audience(strings.ToLower(strings.TrimSpace(value))); audience {
		case AudienceHTTP, AudienceMCP:
			audiences[audience] = true
		case "":
			return nil, errors.New("audience entries must not be empty")
		default:
			return nil, fmt.Errorf("invalid audience %q", value)
		}
	}
	return audiences, nil
}

func bearerToken(header string) (string, bool) {
	fields := strings.Fields(header)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "bearer") {
		return "", false
	}
	return fields[1], true
}

func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func ValidScope(scope Scope) bool {
	if scope == ScopeAll {
		return true
	}
	verb, resource, ok := parseScope(scope)
	return ok && (verb == "read" || verb == "write") && resource != ""
}

func ScopeAllows(grant Scope, required Scope) bool {
	if required == "" || grant == ScopeAll || grant == required {
		return true
	}
	grantVerb, grantResource, ok := parseScope(grant)
	if !ok || grantResource != "*" {
		return false
	}
	requiredVerb, _, ok := parseScope(required)
	return ok && grantVerb == requiredVerb
}

func scopesAllow(scopes []Scope, required Scope) bool {
	for _, scope := range scopes {
		if ScopeAllows(scope, required) {
			return true
		}
	}
	return false
}

func scopesFromStrings(values []string) []Scope {
	scopes := make([]Scope, 0, len(values))
	for _, value := range values {
		scopes = append(scopes, Scope(value))
	}
	return scopes
}

func scopeStrings(scopes []Scope) []string {
	values := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		values = append(values, string(scope))
	}
	return values
}

func parseScope(scope Scope) (verb string, resource string, ok bool) {
	value := string(scope)
	if value == "" || strings.ContainsAny(value, " \t\r\n") || strings.Count(value, ":") != 1 {
		return "", "", false
	}
	verb, resource, ok = strings.Cut(value, ":")
	return verb, resource, ok
}
