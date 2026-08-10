package handlers

import (
	"testing"
)

func TestSafeRedirectPath(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", "/"},
		{"root", "/", "/"},
		{"simple path", "/anime/123", "/anime/123"},
		{"path with query", "/entry?id=1", "/entry?id=1"},
		{"protocol-relative", "//evil.example", "/"},
		{"absolute http", "http://evil.example", "/"},
		{"absolute https", "https://evil.example/x", "/"},
		{"scheme smuggling", "javascript:alert(1)", "/"},
		{"backslash smuggling", "/\\evil.example", "/"},
		{"relative path", "anime/123", "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := safeRedirectPath(tt.input); got != tt.want {
				t.Errorf("safeRedirectPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsOidcIdentityAllowed(t *testing.T) {
	tests := []struct {
		name             string
		subject          string
		username         string
		allowedSubjects  []string
		allowedUsernames []string
		want             bool
	}{
		{"subject match", "sub-1", "alice", []string{"sub-1"}, nil, true},
		{"subject mismatch", "sub-2", "alice", []string{"sub-1"}, nil, false},
		{"username exact match", "sub-9", "alice", nil, []string{"alice"}, true},
		{"username case-insensitive match", "sub-9", "Alice", nil, []string{"aLiCe"}, true},
		{"username mismatch", "sub-9", "bob", nil, []string{"alice"}, false},
		{"empty username never matches", "sub-9", "", nil, []string{""}, false},
		{"both empty allowlists reject", "sub-9", "alice", nil, nil, false},
		{"subject wins over username miss", "sub-1", "nobody", []string{"sub-1"}, []string{"alice"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isOidcIdentityAllowed(tt.subject, tt.username, tt.allowedSubjects, tt.allowedUsernames); got != tt.want {
				t.Errorf("isOidcIdentityAllowed(%q, %q, %v, %v) = %v, want %v",
					tt.subject, tt.username, tt.allowedSubjects, tt.allowedUsernames, got, tt.want)
			}
		})
	}
}
