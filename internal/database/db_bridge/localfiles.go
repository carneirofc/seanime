package db_bridge

import (
	"errors"
	"seanime/internal/database/db"
	"seanime/internal/database/models"
	"seanime/internal/library/anime"
	"sync"

	"github.com/goccy/go-json"
	"github.com/samber/mo"
	"gorm.io/gorm"
)

// The local files are held as a single published snapshot, guarded by localFilesMu.
//
// Nothing mutates a *anime.LocalFile once it has been published, so readers can share the slice
// without copying it. Everything that changes a local file goes through MutateLocalFiles, which
// copies, mutates the copy, persists it and publishes the result. Before that, callers read the
// shared slice, mutated the shared LocalFile values in place and each wrote the whole blob back
// under the id they had read — a data race on these globals, and one caller's change silently
// dropped whenever two overlapped.
var (
	localFilesMu       sync.RWMutex
	CurrLocalFilesDbId uint
	CurrLocalFiles     mo.Option[[]*anime.LocalFile]
)

// GetLocalFiles returns the current local files and the id of the database entry they came from.
//
// The returned slice is a snapshot and must be treated as read-only. To change a local file, use
// MutateLocalFiles.
func GetLocalFiles(db *db.Database) ([]*anime.LocalFile, uint, error) {
	localFilesMu.RLock()
	if CurrLocalFiles.IsPresent() {
		lfs, lfsId := CurrLocalFiles.MustGet(), CurrLocalFilesDbId
		localFilesMu.RUnlock()
		return lfs, lfsId, nil
	}
	localFilesMu.RUnlock()

	localFilesMu.Lock()
	defer localFilesMu.Unlock()
	return loadLocalFilesLocked(db)
}

// loadLocalFilesLocked returns the published snapshot, reading it from the database when there is
// not one yet. The caller must hold the write lock.
func loadLocalFilesLocked(db *db.Database) ([]*anime.LocalFile, uint, error) {
	// Another caller may have published one while this one waited for the lock.
	if CurrLocalFiles.IsPresent() {
		return CurrLocalFiles.MustGet(), CurrLocalFilesDbId, nil
	}

	// Get the latest entry
	var res models.LocalFiles
	err := db.Gorm().Last(&res).Error
	if err != nil {
		return nil, 0, err
	}

	// Unmarshal the local files
	lfsBytes := res.Value
	var lfs []*anime.LocalFile
	if err := json.Unmarshal(lfsBytes, &lfs); err != nil {
		return nil, 0, err
	}

	db.Logger.Debug().Msg("db: Local files retrieved")

	CurrLocalFiles = mo.Some(lfs)
	CurrLocalFilesDbId = res.ID

	return lfs, res.ID, nil
}

// MutateLocalFiles applies mutate to a private deep copy of the local files, persists the result
// and publishes it. The whole read-modify-write is serialized, so two concurrent callers can no
// longer each save a blob built from the same starting point and lose one of the two changes.
//
// mutate returns the slice to save, so it may add, drop or reorder entries. It runs under the
// write lock: keep slow work (network calls, metadata hydration) outside it and pass in the result.
func MutateLocalFiles(db *db.Database, mutate func(lfs []*anime.LocalFile) ([]*anime.LocalFile, error)) ([]*anime.LocalFile, error) {
	localFilesMu.Lock()
	defer localFilesMu.Unlock()

	lfs, lfsId, err := loadLocalFilesLocked(db)
	if err != nil {
		return nil, err
	}

	next, err := mutate(anime.CloneLocalFiles(lfs))
	if err != nil {
		return nil, err
	}

	return saveLocalFilesLocked(db, lfsId, next)
}

// SaveLocalFiles will save the local files in the database at the given id.
//
// It publishes lfs as the new snapshot, so the caller must not retain or mutate it afterwards.
// Use MutateLocalFiles for a read-modify-write: reading with GetLocalFiles and saving the result
// here is the pattern that lost one of two concurrent changes.
func SaveLocalFiles(db *db.Database, lfsId uint, lfs []*anime.LocalFile) ([]*anime.LocalFile, error) {
	localFilesMu.Lock()
	defer localFilesMu.Unlock()
	return saveLocalFilesLocked(db, lfsId, lfs)
}

// saveLocalFilesLocked persists and publishes lfs. The caller must hold the write lock.
func saveLocalFilesLocked(db *db.Database, lfsId uint, lfs []*anime.LocalFile) ([]*anime.LocalFile, error) {
	// Marshal the local files
	marshaledLfs, err := json.Marshal(lfs)
	if err != nil {
		return nil, err
	}

	// Save the local files
	ret, err := db.UpsertLocalFiles(&models.LocalFiles{
		BaseModel: models.BaseModel{
			ID: lfsId,
		},
		Value: marshaledLfs,
	})
	if err != nil {
		return nil, err
	}

	// Unmarshal the saved local files
	var retLfs []*anime.LocalFile
	if err := json.Unmarshal(ret.Value, &retLfs); err != nil {
		CurrLocalFiles = mo.Some(lfs)
		CurrLocalFilesDbId = lfsId
		return lfs, nil
	}

	CurrLocalFiles = mo.Some(retLfs)
	CurrLocalFilesDbId = ret.ID

	return retLfs, nil
}

// InsertLocalFiles will insert the local files in the database at a new entry.
//
// It publishes lfs as the new snapshot, so the caller must not retain or mutate it afterwards.
func InsertLocalFiles(db *db.Database, lfs []*anime.LocalFile) ([]*anime.LocalFile, error) {
	localFilesMu.Lock()
	defer localFilesMu.Unlock()

	// Marshal the local files
	bytes, err := json.Marshal(lfs)
	if err != nil {
		return nil, err
	}

	// Save the local files to the database
	ret, err := db.InsertLocalFiles(&models.LocalFiles{
		Value: bytes,
	})

	if err != nil {
		return nil, err
	}

	CurrLocalFiles = mo.Some(lfs)
	CurrLocalFilesDbId = ret.ID

	return lfs, nil

}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func GetShelvedLocalFiles(db *db.Database) ([]*anime.LocalFile, error) {
	var res models.ShelvedLocalFiles
	err := db.Gorm().Last(&res).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	lfsBytes := res.Value
	var lfs []*anime.LocalFile
	if err := json.Unmarshal(lfsBytes, &lfs); err != nil {
		return nil, err
	}

	db.Logger.Debug().Msg("db: Shelved local files retrieved")

	return lfs, nil
}

func SaveShelvedLocalFiles(db *db.Database, lfs []*anime.LocalFile) error {
	// Marshal the local files
	marshaledLfs, err := json.Marshal(lfs)
	if err != nil {
		return err
	}

	// Save the local files
	ret, err := db.UpsertShelvedLocalFiles(&models.ShelvedLocalFiles{
		BaseModel: models.BaseModel{
			ID: 1,
		},
		Value: marshaledLfs,
	})
	if err != nil {
		return err
	}

	// Unmarshal the saved local files
	var retLfs []*anime.LocalFile
	if err := json.Unmarshal(ret.Value, &retLfs); err != nil {
		return nil
	}

	return nil
}
