package db_bridge

import (
	"fmt"
	"strconv"
	"sync"
	"testing"

	"seanime/internal/database/db"
	"seanime/internal/library/anime"
	"seanime/internal/util"

	"github.com/samber/mo"
	"github.com/stretchr/testify/require"
)

func newLocalFilesTestDatabase(t *testing.T) *db.Database {
	t.Helper()

	database, err := db.NewDatabase(t.TempDir(), "localfiles_test", util.NewLogger())
	require.NoError(t, err)

	// The snapshot is package state shared by every test in this binary.
	localFilesMu.Lock()
	CurrLocalFiles = mo.None[[]*anime.LocalFile]()
	CurrLocalFilesDbId = 0
	localFilesMu.Unlock()

	t.Cleanup(func() {
		localFilesMu.Lock()
		CurrLocalFiles = mo.None[[]*anime.LocalFile]()
		CurrLocalFilesDbId = 0
		localFilesMu.Unlock()
	})

	return database
}

func newTestLocalFiles(count int) []*anime.LocalFile {
	lfs := make([]*anime.LocalFile, count)
	for i := range lfs {
		lfs[i] = &anime.LocalFile{
			Path:       fmt.Sprintf("/library/show/episode-%d.mkv", i),
			Name:       fmt.Sprintf("episode-%d.mkv", i),
			ParsedData: &anime.LocalFileParsedData{Original: strconv.Itoa(i)},
			Metadata:   &anime.LocalFileMetadata{Episode: i, Type: anime.LocalFileTypeMain},
		}
	}
	return lfs
}

func TestMutateLocalFilesKeepsConcurrentChanges(t *testing.T) {
	// Every caller used to read the same slice, mutate the same shared *LocalFile values and write
	// the whole blob back, so overlapping requests silently dropped one of the two changes.
	database := newLocalFilesTestDatabase(t)

	const count = 16
	_, err := InsertLocalFiles(database, newTestLocalFiles(count))
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := MutateLocalFiles(database, func(lfs []*anime.LocalFile) ([]*anime.LocalFile, error) {
				lfs[i].MediaId = 1000 + i
				return lfs, nil
			})
			require.NoError(t, err)
		}()
	}
	wg.Wait()

	lfs, _, err := GetLocalFiles(database)
	require.NoError(t, err)
	require.Len(t, lfs, count)
	for i, lf := range lfs {
		require.Equal(t, 1000+i, lf.MediaId, "every concurrent change should survive")
	}
}

func TestGetLocalFilesIsSafeToReadWhileMutating(t *testing.T) {
	// Run under -race: readers hold the published snapshot after the call returns, so a mutation
	// must build a copy rather than edit the values they are reading.
	database := newLocalFilesTestDatabase(t)

	_, err := InsertLocalFiles(database, newTestLocalFiles(32))
	require.NoError(t, err)

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 25 {
				lfs, _, err := GetLocalFiles(database)
				require.NoError(t, err)
				for _, lf := range lfs {
					_ = lf.MediaId
					_ = lf.Locked
					if lf.Metadata != nil {
						_ = lf.Metadata.Episode
					}
				}
			}
		}()
	}

	for w := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := range 25 {
				_, err := MutateLocalFiles(database, func(lfs []*anime.LocalFile) ([]*anime.LocalFile, error) {
					for _, lf := range lfs {
						lf.MediaId = w*100 + n
						lf.Locked = n%2 == 0
						if lf.Metadata != nil {
							lf.Metadata.Episode = n
						}
					}
					return lfs, nil
				})
				require.NoError(t, err)
			}
		}()
	}

	wg.Wait()
}

func TestMutateLocalFilesDoesNotPublishOnError(t *testing.T) {
	database := newLocalFilesTestDatabase(t)

	_, err := InsertLocalFiles(database, newTestLocalFiles(3))
	require.NoError(t, err)

	_, err = MutateLocalFiles(database, func(lfs []*anime.LocalFile) ([]*anime.LocalFile, error) {
		lfs[0].MediaId = 42
		return nil, fmt.Errorf("nope")
	})
	require.Error(t, err)

	lfs, _, err := GetLocalFiles(database)
	require.NoError(t, err)
	require.Zero(t, lfs[0].MediaId, "a failed mutation must not reach the published snapshot")
}
