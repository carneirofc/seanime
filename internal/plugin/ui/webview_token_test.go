package plugin_ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// State sent through a webview channel is rendered by plugin-controlled HTML, so the
// AniList token must never travel with it. The masking used to be written as a bare
// strings.ReplaceAll whose result was thrown away, which let string payloads through
// untouched.
func TestScrubAnilistToken(t *testing.T) {
	const token = "anilist-access-token"

	t.Run("masks a token inside a string", func(t *testing.T) {
		value, ok := scrubAnilistToken("Bearer "+token, token)

		assert.True(t, ok)
		assert.Equal(t, "Bearer [TOKEN]", value)
	})

	t.Run("refuses a structured value carrying the token", func(t *testing.T) {
		_, ok := scrubAnilistToken(map[string]string{"auth": token}, token)

		assert.False(t, ok)
	})

	t.Run("passes values that do not carry the token", func(t *testing.T) {
		payload := map[string]any{"episode": float64(12)}

		value, ok := scrubAnilistToken(payload, token)

		assert.True(t, ok)
		assert.Equal(t, payload, value)
	})

	// With no account connected the token is empty, and strings.Contains reports the
	// empty string as present in everything — which used to drop every message.
	t.Run("passes everything when no token is set", func(t *testing.T) {
		str, ok := scrubAnilistToken("plain message", "")
		assert.True(t, ok)
		assert.Equal(t, "plain message", str)

		payload := map[string]any{"episode": float64(12)}
		value, ok := scrubAnilistToken(payload, "")
		assert.True(t, ok)
		assert.Equal(t, payload, value)
	})
}
