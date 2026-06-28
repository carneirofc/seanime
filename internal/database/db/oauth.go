package db

import (
	"errors"
	"seanime/internal/database/models"
	"time"

	"gorm.io/gorm"
)

// -----------------------------------------------------------------------
// OAuthClient
// -----------------------------------------------------------------------

func (db *Database) GetOAuthClientByClientID(clientID string) (*models.OAuthClient, error) {
	var client models.OAuthClient
	err := db.gormdb.Where("client_id = ?", clientID).First(&client).Error
	if err != nil {
		return nil, err
	}
	return &client, nil
}

func (db *Database) ListOAuthClients() ([]*models.OAuthClient, error) {
	var clients []*models.OAuthClient
	err := db.gormdb.Find(&clients).Error
	return clients, err
}

func (db *Database) CreateOAuthClient(client *models.OAuthClient) (*models.OAuthClient, error) {
	err := db.gormdb.Create(client).Error
	return client, err
}

func (db *Database) DeleteOAuthClient(id uint) error {
	return db.gormdb.Delete(&models.OAuthClient{}, id).Error
}

// -----------------------------------------------------------------------
// OAuthAuthCode
// -----------------------------------------------------------------------

func (db *Database) CreateOAuthAuthCode(code *models.OAuthAuthCode) (*models.OAuthAuthCode, error) {
	err := db.gormdb.Create(code).Error
	return code, err
}

// ConsumeOAuthAuthCode looks up the code, validates it (not expired, not used,
// correct client), marks it as used, and returns it.
func (db *Database) ConsumeOAuthAuthCode(code, clientID, redirectURI string) (*models.OAuthAuthCode, error) {
	var authCode models.OAuthAuthCode
	err := db.gormdb.Where("code = ? AND client_id = ? AND used = ? AND expires_at > ?",
		code, clientID, false, time.Now(),
	).First(&authCode).Error
	if err != nil {
		return nil, errors.New("invalid or expired authorization code")
	}
	if redirectURI != "" && authCode.RedirectURI != redirectURI {
		return nil, errors.New("redirect_uri mismatch")
	}
	authCode.Used = true
	db.gormdb.Save(&authCode)
	return &authCode, nil
}

// -----------------------------------------------------------------------
// OAuthAccessToken
// -----------------------------------------------------------------------

func (db *Database) CreateOAuthAccessToken(token *models.OAuthAccessToken) (*models.OAuthAccessToken, error) {
	err := db.gormdb.Create(token).Error
	return token, err
}

// GetValidOAuthAccessToken returns the token record if it exists, is not
// revoked, and has not expired.
func (db *Database) GetValidOAuthAccessToken(accessToken string) (*models.OAuthAccessToken, error) {
	var t models.OAuthAccessToken
	err := db.gormdb.Where("access_token = ? AND revoked = ?", accessToken, false).First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("token not found")
		}
		return nil, err
	}
	if t.ExpiresAt != nil && time.Now().After(*t.ExpiresAt) {
		return nil, errors.New("token expired")
	}
	return &t, nil
}

// GetOAuthAccessTokenByRefresh returns the token record matching a refresh token.
func (db *Database) GetOAuthAccessTokenByRefresh(refreshToken string) (*models.OAuthAccessToken, error) {
	var t models.OAuthAccessToken
	err := db.gormdb.Where("refresh_token = ? AND revoked = ?", refreshToken, false).First(&t).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// RevokeOAuthAccessToken marks access and (optionally) refresh tokens as revoked.
func (db *Database) RevokeOAuthAccessToken(token string) error {
	result := db.gormdb.Model(&models.OAuthAccessToken{}).
		Where("access_token = ? OR refresh_token = ?", token, token).
		Update("revoked", true)
	if result.RowsAffected == 0 {
		return errors.New("token not found")
	}
	return result.Error
}

// RevokeAllOAuthTokensForClient revokes every token issued to a given client.
func (db *Database) RevokeAllOAuthTokensForClient(clientID string) error {
	return db.gormdb.Model(&models.OAuthAccessToken{}).
		Where("client_id = ?", clientID).
		Update("revoked", true).Error
}
