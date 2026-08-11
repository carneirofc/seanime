package db

import (
	"seanime/internal/database/models"
	"seanime/internal/util"
	"testing"
	"time"
)

func newSessionTestDatabase(t *testing.T) *Database {
	t.Helper()

	logger := util.NewLogger()
	database, err := NewDatabase(t.TempDir(), "server_session_test", logger)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	return database
}

func TestServerSessionLifecycle(t *testing.T) {
	database := newSessionTestDatabase(t)

	hash := util.HashSHA256Hex("raw-token")
	_, err := database.CreateServerSession(&models.ServerSession{
		TokenHash:  hash,
		Subject:    "sub-1",
		Username:   "alice",
		ExpiresAt:  time.Now().Add(time.Hour),
		LastSeenAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("CreateServerSession: %v", err)
	}

	session, err := database.GetValidServerSession(hash)
	if err != nil {
		t.Fatalf("GetValidServerSession: %v", err)
	}
	if session.Subject != "sub-1" || session.Username != "alice" {
		t.Errorf("unexpected session data: %+v", session)
	}

	if _, err := database.GetValidServerSession(util.HashSHA256Hex("other")); err == nil {
		t.Error("expected error for unknown token hash")
	}

	newExpiry := time.Now().Add(2 * time.Hour)
	if err := database.TouchServerSession(session.ID, time.Now(), newExpiry); err != nil {
		t.Fatalf("TouchServerSession: %v", err)
	}
	touched, err := database.GetValidServerSession(hash)
	if err != nil {
		t.Fatalf("GetValidServerSession after touch: %v", err)
	}
	if touched.ExpiresAt.Before(session.ExpiresAt) {
		t.Error("expected expiry to move forward after touch")
	}

	if err := database.DeleteServerSession(hash); err != nil {
		t.Fatalf("DeleteServerSession: %v", err)
	}
	if _, err := database.GetValidServerSession(hash); err == nil {
		t.Error("expected error after deletion")
	}
}

// Media tokens are bound to a session by id, and the binding is only worth
// anything if the lookup honours expiry and deletion the same way the cookie path
// does — otherwise signing out leaves every minted media URL live.
func TestGetValidServerSessionByID(t *testing.T) {
	database := newSessionTestDatabase(t)

	hash := util.HashSHA256Hex("raw-token")
	created, err := database.CreateServerSession(&models.ServerSession{
		TokenHash:  hash,
		Subject:    "sub-1",
		Username:   "alice",
		ExpiresAt:  time.Now().Add(time.Hour),
		LastSeenAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("CreateServerSession: %v", err)
	}

	session, err := database.GetValidServerSessionByID(created.ID)
	if err != nil {
		t.Fatalf("GetValidServerSessionByID: %v", err)
	}
	if session.Username != "alice" {
		t.Errorf("unexpected session data: %+v", session)
	}

	if _, err := database.GetValidServerSessionByID(created.ID + 1000); err == nil {
		t.Error("expected error for an unknown session id")
	}

	if err := database.DeleteServerSession(hash); err != nil {
		t.Fatalf("DeleteServerSession: %v", err)
	}
	if _, err := database.GetValidServerSessionByID(created.ID); err == nil {
		t.Error("a logged-out session must not resolve by id")
	}
}

func TestGetValidServerSessionByIDRejectsExpired(t *testing.T) {
	database := newSessionTestDatabase(t)

	created, err := database.CreateServerSession(&models.ServerSession{
		TokenHash:  util.HashSHA256Hex("expired"),
		Subject:    "sub-2",
		ExpiresAt:  time.Now().Add(-time.Minute),
		LastSeenAt: time.Now().Add(-time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateServerSession: %v", err)
	}

	if _, err := database.GetValidServerSessionByID(created.ID); err == nil {
		t.Error("expected an expired session to be rejected by id")
	}
}

func TestServerSessionExpiry(t *testing.T) {
	database := newSessionTestDatabase(t)

	expiredHash := util.HashSHA256Hex("expired")
	_, err := database.CreateServerSession(&models.ServerSession{
		TokenHash:  expiredHash,
		Subject:    "sub-2",
		ExpiresAt:  time.Now().Add(-time.Minute),
		LastSeenAt: time.Now().Add(-time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateServerSession: %v", err)
	}

	if _, err := database.GetValidServerSession(expiredHash); err == nil {
		t.Error("expected expired session to be rejected")
	}

	validHash := util.HashSHA256Hex("valid")
	if _, err := database.CreateServerSession(&models.ServerSession{
		TokenHash:  validHash,
		Subject:    "sub-3",
		ExpiresAt:  time.Now().Add(time.Hour),
		LastSeenAt: time.Now(),
	}); err != nil {
		t.Fatalf("CreateServerSession: %v", err)
	}

	deleted, err := database.DeleteExpiredServerSessions()
	if err != nil {
		t.Fatalf("DeleteExpiredServerSessions: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 expired session deleted, got %d", deleted)
	}

	if _, err := database.GetValidServerSession(validHash); err != nil {
		t.Errorf("valid session must survive GC: %v", err)
	}

	if err := database.DeleteAllServerSessions(); err != nil {
		t.Fatalf("DeleteAllServerSessions: %v", err)
	}
	if _, err := database.GetValidServerSession(validHash); err == nil {
		t.Error("expected all sessions gone after DeleteAllServerSessions")
	}
}
