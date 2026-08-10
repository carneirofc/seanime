package events

import (
	"testing"

	"seanime/internal/util"

	"github.com/stretchr/testify/require"
)

func TestWSEventManagerGetClientPlatform(t *testing.T) {
	manager := NewWSEventManager(util.NewLogger())

	manager.AddConn("web-client", nil)
	manager.AddConn("mobile-client", nil, "mobile")

	require.Empty(t, manager.GetClientPlatform("web-client"))
	require.Equal(t, "mobile", manager.GetClientPlatform("mobile-client"))
	require.Empty(t, manager.GetClientPlatform("missing-client"))
}
