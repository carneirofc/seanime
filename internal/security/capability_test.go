package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseCapabilities(t *testing.T) {
	t.Run("normalizes, sorts and de-duplicates", func(t *testing.T) {
		got, unknown := ParseCapabilities([]string{" EXEC ", "exec", "filesystem"})
		assert.Equal(t, []string{CapabilityExec, CapabilityFilesystem}, got)
		assert.Empty(t, unknown)
	})

	t.Run("expands all and ignores none", func(t *testing.T) {
		got, _ := ParseCapabilities([]string{"all"})
		assert.Equal(t, AllCapabilities, got)

		got, _ = ParseCapabilities([]string{"none"})
		assert.Empty(t, got)
	})

	t.Run("reports unknown entries instead of guessing", func(t *testing.T) {
		got, unknown := ParseCapabilities([]string{"exec", "root"})
		assert.Equal(t, []string{CapabilityExec}, got)
		assert.Equal(t, []string{"root"}, unknown)
	})
}

func TestResolveDefaultCapabilities(t *testing.T) {
	// A server deployment starts with nothing granted; the operator opts in.
	assert.Empty(t, ResolveDefaultCapabilities(true, "", nil))
	assert.Empty(t, ResolveDefaultCapabilities(false, "https://seanime.example", nil))
	assert.Empty(t, ResolveDefaultCapabilities(false, "", []string{"127.0.0.1"}))

	// A plain local install keeps the historical behaviour.
	assert.Equal(t, AllCapabilities, ResolveDefaultCapabilities(false, "", nil))
}

func TestAllows(t *testing.T) {
	t.Cleanup(func() { SetCapabilities(nil, false) })

	SetCapabilities(nil, true)
	for _, capability := range AllCapabilities {
		assert.False(t, Allows(capability), capability)
	}

	SetCapabilities([]string{CapabilityExec}, true)
	assert.True(t, Allows(CapabilityExec))
	assert.False(t, Allows(CapabilityFilesystem))
	assert.False(t, Allows(CapabilitySelfUpdate))
}

func TestSetCapabilitiesPreservesOtherContextFields(t *testing.T) {
	t.Cleanup(func() {
		SetCapabilities(nil, false)
		SetSecureMode("")
		SetRequestBoundaryConfig(nil, "")
	})

	SetSecureMode(SecureModeStrict)
	SetRequestBoundaryConfig([]string{"10.0.0.1"}, "https://seanime.example")
	SetCapabilities([]string{CapabilityExec}, true)

	assert.True(t, IsStrict())
	assert.Equal(t, []string{"10.0.0.1"}, GetTrustedProxies())
	assert.Equal(t, "https://seanime.example", GetExternalURL())
	assert.Equal(t, []string{CapabilityExec}, GetCapabilities())
}
