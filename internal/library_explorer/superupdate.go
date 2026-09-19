package library_explorer

import (
	"fmt"
	"os"
	"path/filepath"
	"seanime/internal/database/db_bridge"
	"seanime/internal/library/anime"
	"seanime/internal/security"
	"strings"
	"sync"

	"github.com/samber/lo"
)

type SuperUpdateFileOptions struct {
	// Path to the file
	Path string `json:"path"`
	// New name of the file
	NewName string `json:"newName,omitempty"`
	// Metadata of the file
	Metadata *anime.LocalFileMetadata `json:"metadata,omitempty"`
}

func (l *LibraryExplorer) SuperUpdateFiles(opts []*SuperUpdateFileOptions) error {

	const MaxConcurrentUpdates = 10
	sem := make(chan struct{}, MaxConcurrentUpdates)

	l.logger.Debug().
		Int("count", len(opts)).
		Msg("library explorer: Updating files")

	settings, err := l.database.GetSettings()
	if err != nil {
		return err
	}

	// The whole batch runs inside the mutation: the workers below rename files on disk and edit
	// the matching local files in place, so they have to be working on a private copy that nothing
	// else can read, and the result has to be saved from that same copy.
	_, err = db_bridge.MutateLocalFiles(l.database, func(lfs []*anime.LocalFile) ([]*anime.LocalFile, error) {
		for _, opt := range opts {
			lf, err := validateSuperUpdateFile(opt, lfs)
			if err != nil {
				return nil, err
			}
			opt.Path = lf.Path
		}

		wg := sync.WaitGroup{}
		wg.Add(len(opts))

		for _, opt := range opts {
			go func(opt *SuperUpdateFileOptions) {
				sem <- struct{}{}
				defer func() { <-sem }()
				defer wg.Done()
				_ = l.superUpdateFile(opt, lfs, settings.GetLibrary().GetLibraryPaths())
			}(opt)
		}

		wg.Wait()

		return lfs, nil
	})
	if err != nil {
		return err
	}

	l.fileTree = nil

	return nil
}

func validateSuperUpdateFile(opt *SuperUpdateFileOptions, lfs []*anime.LocalFile) (*anime.LocalFile, error) {
	if opt == nil {
		return nil, fmt.Errorf("missing local file")
	}

	lf, found := lo.Find(lfs, func(i *anime.LocalFile) bool {
		return i.HasSamePath(opt.Path)
	})
	if !found {
		return nil, fmt.Errorf("local file not found: %s", opt.Path)
	}

	if opt.NewName != "" && !isValidSuperUpdateName(opt.NewName) {
		return nil, fmt.Errorf("invalid file name: %s", opt.NewName)
	}

	return lf, nil
}

func isValidSuperUpdateName(name string) bool {
	if strings.TrimSpace(name) == "" || name == "." || name == ".." {
		return false
	}
	if filepath.IsAbs(name) || filepath.Base(name) != name {
		return false
	}

	return !strings.ContainsAny(name, `/\\`)
}

func (l *LibraryExplorer) superUpdateFile(opt *SuperUpdateFileOptions, lfs []*anime.LocalFile, libraryPaths []string) error {

	l.logger.Debug().
		Any("path", opt.Path).
		Msg("library explorer: Updating file")

	lf, found := lo.Find(lfs, func(i *anime.LocalFile) bool {
		return i.HasSamePath(opt.Path)
	})
	if security.IsStrict() { // in strict mode, only allow updates to files that are already known by the scanner
		if !found {
			return fmt.Errorf("local file not found: %s", opt.Path)
		}
	}

	if opt.NewName != "" {
		newPath := filepath.Join(filepath.Dir(opt.Path), opt.NewName)
		// Update the file name
		// If the local file exists, update the name
		if found {
			lf.Name = opt.NewName
			// Update the parsed info
			newLf := anime.NewLocalFileS(newPath, libraryPaths)
			lf.ParsedData = newLf.ParsedData
			lf.ParsedFolderData = newLf.ParsedFolderData
			lf.Path = newPath
		}

		// Rename the real file name
		err := os.Rename(opt.Path, newPath)
		if err != nil {
			return err
		}
	}

	if opt.Metadata != nil {
		l.logger.Debug().
			Any("path", opt.Path).
			Any("metadata", opt.Metadata).
			Msg("library explorer: Updating file metadata")
		if found {
			lf.Metadata = opt.Metadata
			lf.Locked = true
			lf.Ignored = false
		}
	}

	return nil
}
