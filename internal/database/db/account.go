package db

import (
	"errors"
	"seanime/internal/database/models"
	"sync"

	"gorm.io/gorm/clause"
)

// accountCache holds the logged-in account, including the AniList access token.
// It is written on login/logout and read by request handlers, so it is guarded:
// without the mutex the login write races every concurrent reader.
var (
	accountCacheMu sync.RWMutex
	accountCache   *models.Account
)

func setAccountCache(acc *models.Account) {
	accountCacheMu.Lock()
	defer accountCacheMu.Unlock()
	accountCache = acc
}

func getAccountCache() *models.Account {
	accountCacheMu.RLock()
	defer accountCacheMu.RUnlock()
	return accountCache
}

func (db *Database) UpsertAccount(acc *models.Account) (*models.Account, error) {
	err := db.gormdb.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		UpdateAll: true,
	}).Create(acc).Error

	if err != nil {
		db.Logger.Error().Err(err).Msg("Failed to save account in the database")
		return nil, err
	}

	if acc.Username != "" {
		setAccountCache(acc)
	} else {
		setAccountCache(nil)
	}

	return acc, nil
}

func (db *Database) GetAccount() (*models.Account, error) {

	if cached := getAccountCache(); cached != nil {
		return cached, nil
	}

	var acc models.Account
	err := db.gormdb.Last(&acc).Error
	if err != nil {
		return nil, err
	}
	if acc.Username == "" || acc.Token == "" || acc.Viewer == nil {
		return nil, errors.New("account not found")
	}

	setAccountCache(&acc)

	return &acc, err
}

// GetAnilistToken retrieves the AniList token from the account or returns an empty string
func (db *Database) GetAnilistToken() string {
	acc, err := db.GetAccount()
	if err != nil {
		return ""
	}
	return acc.Token
}
