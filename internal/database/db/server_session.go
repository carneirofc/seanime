package db

import (
	"errors"
	"seanime/internal/database/models"
	"time"

	"gorm.io/gorm"
)

// -----------------------------------------------------------------------
// ServerSession (OIDC login sessions)
// -----------------------------------------------------------------------

func (db *Database) CreateServerSession(session *models.ServerSession) (*models.ServerSession, error) {
	err := db.gormdb.Create(session).Error
	return session, err
}

// GetValidServerSession returns the session matching the token hash if it has not expired.
func (db *Database) GetValidServerSession(tokenHash string) (*models.ServerSession, error) {
	var session models.ServerSession
	err := db.gormdb.Where("token_hash = ? AND expires_at > ?", tokenHash, time.Now()).First(&session).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("session not found or expired")
		}
		return nil, err
	}
	return &session, nil
}

// TouchServerSession updates the session's last-seen time and sliding expiry.
func (db *Database) TouchServerSession(id uint, lastSeen time.Time, newExpiry time.Time) error {
	return db.gormdb.Model(&models.ServerSession{}).
		Where("id = ?", id).
		Updates(map[string]any{"last_seen_at": lastSeen, "expires_at": newExpiry}).Error
}

func (db *Database) DeleteServerSession(tokenHash string) error {
	return db.gormdb.Where("token_hash = ?", tokenHash).Delete(&models.ServerSession{}).Error
}

// DeleteExpiredServerSessions removes sessions past their expiry and returns the count.
func (db *Database) DeleteExpiredServerSessions() (int64, error) {
	result := db.gormdb.Where("expires_at <= ?", time.Now()).Delete(&models.ServerSession{})
	return result.RowsAffected, result.Error
}

// DeleteAllServerSessions logs every user out (e.g. after a secret rotation).
func (db *Database) DeleteAllServerSessions() error {
	return db.gormdb.Where("1 = 1").Delete(&models.ServerSession{}).Error
}
