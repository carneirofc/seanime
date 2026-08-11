package security

import (
	"seanime/internal/util"
	"slices"
	"strings"
)

const (
	SecureModeDefault  = ""
	SecureModeHardened = "hardened"
	SecureModeLax      = "lax"
	SecureModeStrict   = "strict"
)

type SecurityContext struct {
	SecureMode     string
	TrustedProxies []string
	ExternalURL    string
	// Capabilities is the set of privileged actions the operator has granted.
	// See capability.go — it is never derived from a request.
	Capabilities []string
	// CapabilitiesConfigured distinguishes "operator wrote an empty list" (deny
	// everything) from "operator wrote nothing" (fall back to the posture default).
	CapabilitiesConfigured bool
}

var GlobalSecurityContext = util.NewRef(&SecurityContext{})

// cloneSecurityContext copies the context so the setters below can replace a
// single field without racing readers of the others.
func cloneSecurityContext(current *SecurityContext) *SecurityContext {
	if current == nil {
		return &SecurityContext{}
	}

	return &SecurityContext{
		SecureMode:             current.SecureMode,
		TrustedProxies:         slices.Clone(current.TrustedProxies),
		ExternalURL:            current.ExternalURL,
		Capabilities:           slices.Clone(current.Capabilities),
		CapabilitiesConfigured: current.CapabilitiesConfigured,
	}
}

func NormalizeMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case SecureModeHardened:
		return SecureModeHardened
	case SecureModeLax:
		return SecureModeLax
	case SecureModeStrict:
		return SecureModeStrict
	default:
		return SecureModeDefault
	}
}

func SetSecureMode(mode string) {
	next := cloneSecurityContext(GlobalSecurityContext.Get())
	next.SecureMode = NormalizeMode(mode)
	GlobalSecurityContext.Set(next)
}

func IsStrict() bool {
	return GlobalSecurityContext.Get().SecureMode == SecureModeStrict
}

func IsLax() bool {
	return GlobalSecurityContext.Get().SecureMode == SecureModeLax
}

func IsHardened() bool {
	mode := GlobalSecurityContext.Get().SecureMode
	return mode == SecureModeHardened || mode == SecureModeStrict
}

func SetRequestBoundaryConfig(trustedProxies []string, externalURL string) {
	next := cloneSecurityContext(GlobalSecurityContext.Get())
	next.TrustedProxies = slices.Clone(trustedProxies)
	next.ExternalURL = strings.TrimSpace(externalURL)
	GlobalSecurityContext.Set(next)
}

func GetTrustedProxies() []string {
	return slices.Clone(GlobalSecurityContext.Get().TrustedProxies)
}

func GetExternalURL() string {
	return strings.TrimSpace(GlobalSecurityContext.Get().ExternalURL)
}
