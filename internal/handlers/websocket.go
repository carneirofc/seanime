package handlers

import (
	"net/http"
	"seanime/internal/events"
	"seanime/internal/security"
	"strings"

	"github.com/goccy/go-json"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(*http.Request) bool { return false },
	}
)

// webSocketEventHandler creates a new websocket handler for real-time event communication
func (h *Handler) webSocketEventHandler(c echo.Context) error {
	req := c.Request()
	if !websocketUpgradeRateLimits.allow(websocketUpgradeRateLimitKey(req), maxWebsocketAttemptsPerWindow, websocketUpgradeWindow) {
		return c.JSON(http.StatusTooManyRequests, NewErrorResponse(errTooManyRequests))
	}

	if h.App.IsOidcMode() {
		// The session cookie rides along on the websocket upgrade request;
		// no query-param token is involved in OIDC mode.
		if _, ok := h.resolveServerSession(req); !ok {
			authKey := authFailureRateLimitKey(req)
			if !authFailureRateLimits.allow(authKey, maxAuthFailuresPerWindow, authFailureWindow) {
				return c.JSON(http.StatusTooManyRequests, NewErrorResponse(errTooManyAuthenticationAttempts))
			}
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		}
		authFailureRateLimits.reset(authFailureRateLimitKey(req))
	} else if h.App.Config.Server.Password != "" {
		// When a server password is set, require auth via query param -- a browser
		// cannot set headers on an upgrade request, so this is the only channel.
		//
		// A query string is logged by the reverse proxy in front of us, so what
		// travels here should not be the credential itself: clients exchange it for
		// a short-lived ticket at /api/v1/auth/ws-ticket and send that. The raw
		// password hash is still accepted, for non-browser clients (the mobile
		// build) that connect without minting one first.
		token := c.QueryParam("token")
		if !h.App.ValidateWebsocketTicket(token) && !secureCompare(token, h.App.ServerPasswordHash) {
			authKey := authFailureRateLimitKey(req)
			if !authFailureRateLimits.allow(authKey, maxAuthFailuresPerWindow, authFailureWindow) {
				return c.JSON(http.StatusTooManyRequests, NewErrorResponse(errTooManyAuthenticationAttempts))
			}
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		}

		authFailureRateLimits.reset(authFailureRateLimitKey(req))
	}

	contextClientId := getContextClientId(c)

	if !h.hasServerAuth() {
		if !security.IsLax() && reqHasOriginMetadata(req) && !isRequestFromTrustedOrigin(req) && !isRequestFromAllowlistedOrigin(req, h.App.Config.Server.AccessAllowlist) {
			return c.JSON(http.StatusForbidden, NewErrorResponse(errRequestBoundaryDenied))
		}
	}

	requestUpgrader := upgrader
	requestUpgrader.CheckOrigin = func(r *http.Request) bool {
		if h.App.IsOidcMode() {
			// Cookie-authenticated upgrade: only same-origin browsers (no Origin
			// header never happens on browser WS; non-browser clients are fine),
			// the canonical external origin, or allowlisted origins may connect.
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			if origin == "" {
				return true // non-browser client
			}
			return isTrustedCORSOriginWithCookies(origin, h.App.Config.Server.ExternalURL, h.App.Config.Server.AccessAllowlist)
		}
		return isRequestPermitted(r, h.hasServerAuth(), h.App.Config.Server.AccessAllowlist)
	}

	ws, err := requestUpgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	defer ws.Close()

	// Get connection ID from query parameter
	id := contextClientId
	if id == "" {
		id = "0"
	}
	platform := getClientPlatformFromContext(c)

	// Add connection to manager
	h.App.WSEventManager.AddConn(id, ws, platform)
	h.App.Logger.Debug().Str("id", id).Str("platform", platform).Msg("ws: Client connected")
	h.App.WSEventManager.SendEventTo(id, events.ClientIdentity, map[string]string{
		"clientId": id,
		"proof":    generateClientIdentityProof(h.App, id),
		"platform": platform,
	}, true)

	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				h.App.Logger.Debug().Str("id", id).Msg("ws: Client disconnected")
			} else {
				h.App.Logger.Debug().Str("id", id).Msg("ws: Client disconnection")
			}
			h.App.WSEventManager.RemoveConn(id)
			break
		}

		event, err := UnmarshalWebsocketClientEvent(msg)
		if err != nil {
			h.App.Logger.Error().Err(err).Msg("ws: Failed to unmarshal message sent from webview")
			continue
		}

		event.ClientID = id
		event.Payload = addClientIdToPayload(event.Payload, id)

		// Handle ping messages
		if event.Type == "ping" {
			timestamp := int64(0)
			if payload, ok := event.Payload.(map[string]interface{}); ok {
				if ts, ok := payload["timestamp"]; ok {
					if tsFloat, ok := ts.(float64); ok {
						timestamp = int64(tsFloat)
					} else if tsInt, ok := ts.(int64); ok {
						timestamp = tsInt
					}
				}
			}

			// Send pong response back to the same client
			h.App.WSEventManager.SendEventTo(id, "pong", map[string]int64{"timestamp": timestamp})
			continue // Skip further processing for ping messages
		}

		// Handle main-tab-claim messages by broadcasting to all clients
		if event.Type == "main-tab-claim" {
			h.App.WSEventManager.SendEvent("main-tab-claim", event.Payload)
			continue
		}

		h.HandleClientEvents(event)

		// h.App.Logger.Debug().Msgf("ws: message received: %+v", msg)

		// // Echo the message back
		// if err = ws.WriteMessage(messageType, msg); err != nil {
		// 	h.App.Logger.Err(err).Msg("ws: Failed to send message")
		// 	break
		// }
	}

	return nil
}

func UnmarshalWebsocketClientEvent(msg []byte) (*events.WebsocketClientEvent, error) {
	var event events.WebsocketClientEvent
	if err := json.Unmarshal(msg, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

func addClientIdToPayload(value interface{}, clientID string) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, nested := range typed {
			if key == "clientId" {
				typed[key] = clientID
				continue
			}
			typed[key] = addClientIdToPayload(nested, clientID)
		}
		return typed
	case []interface{}:
		for index, nested := range typed {
			typed[index] = addClientIdToPayload(nested, clientID)
		}
		return typed
	default:
		return value
	}
}
