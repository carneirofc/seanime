package report

import (
	"seanime/internal/database/models"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

// The log content bundled into an issue report is attached to public bug reports,
// so every credential the server holds has to be scrubbed out of it. The AniList
// token is the one that does not live in settings: it sits on the account row, so
// nothing collects it unless the caller passes it in explicitly.
func TestAnonymizeRedactsCredentials(t *testing.T) {
	logger := zerolog.Nop()
	repo := NewRepository(&logger)

	const anilistToken = "eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9.anilist-access-token"

	settings := &models.Settings{
		MediaPlayer: &models.MediaPlayerSettings{
			VlcPassword:       "vlc-secret",
			VcTranslateApiKey: "deepl-api-key",
		},
		Torrent: &models.TorrentSettings{
			QBittorrentPassword: "qbit-secret",
		},
	}

	content := strings.Join([]string{
		`{"level":"error","sql":"INSERT INTO accounts (token) VALUES (\"` + anilistToken + `\")"}`,
		`{"msg":"vlc-secret"}`,
		`{"msg":"deepl-api-key"}`,
		`{"msg":"qbit-secret"}`,
		`{"url":"https://example.com/cb?code=abc123&state=xyz"}`,
	}, "\n")

	out := repo.Anonymize(AnonymizeOptions{
		Content:      []byte(content),
		Settings:     settings,
		AnilistToken: anilistToken,
	})

	assert.NotContains(t, out, anilistToken)
	assert.NotContains(t, out, "vlc-secret")
	assert.NotContains(t, out, "deepl-api-key")
	assert.NotContains(t, out, "qbit-secret")
	assert.NotContains(t, out, "abc123")
	assert.Contains(t, out, "[REDACTED]")
}

// An empty token must not turn into a match-everything replacement.
func TestAnonymizeWithoutAnilistTokenLeavesContentIntact(t *testing.T) {
	logger := zerolog.Nop()
	repo := NewRepository(&logger)

	out := repo.Anonymize(AnonymizeOptions{
		Content:      []byte(`{"msg":"nothing sensitive here"}`),
		AnilistToken: "",
	})

	assert.Equal(t, `{"msg":"nothing sensitive here"}`, out)
}
