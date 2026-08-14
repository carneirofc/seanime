package manga

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"seanime/internal/api/anilist"
	"seanime/internal/extension"
	hibikemanga "seanime/internal/extension/hibike/manga"
	"seanime/internal/hook"
	chapter_downloader "seanime/internal/manga/downloader"
	manga_providers "seanime/internal/manga/providers"
	"slices"
	"strings"

	"github.com/goccy/go-json"
)

// GetDownloadedMangaChapterContainers retrieves downloaded chapter containers for a specific manga ID.
// It filters the complete set of downloaded chapters to return only those matching the provided manga ID.
func (r *Repository) GetDownloadedMangaChapterContainers(mId int, mangaCollection *anilist.MangaCollection) (ret []*ChapterContainer, err error) {

	containers, err := r.GetDownloadedChapterContainers(mangaCollection)
	if err != nil {
		return nil, err
	}

	for _, container := range containers {
		if container.MediaId == mId {
			ret = append(ret, container)
		}
	}

	return ret, nil
}

// GetDownloadedChapterContainers retrieves all downloaded manga chapter containers.
// It scans the download directory for chapter folders, matches them with manga collection entries,
// and collects chapter details from file cache or provider API when necessary.
//
// Ideally, the provider API should never be called assuming the chapter details are cached.
func (r *Repository) GetDownloadedChapterContainers(mangaCollection *anilist.MangaCollection) (ret []*ChapterContainer, err error) {
	ret = make([]*ChapterContainer, 0)

	// Trigger hook event
	reqEvent := &MangaDownloadedChapterContainersRequestedEvent{
		MangaCollection:   mangaCollection,
		ChapterContainers: ret,
	}
	err = hook.GlobalHookManager.OnMangaDownloadedChapterContainersRequested().Trigger(reqEvent)
	if err != nil {
		r.logger.Error().Err(err).Msg("manga: Exception occurred while triggering hook event")
		return nil, fmt.Errorf("manga: Error in hook, %w", err)
	}
	mangaCollection = reqEvent.MangaCollection

	// Default prevented, return the chapter containers
	if reqEvent.DefaultPrevented {
		ret = reqEvent.ChapterContainers
		if ret == nil {
			return nil, fmt.Errorf("manga: No chapter containers returned by hook event")
		}
		return ret, nil
	}

	// Scan the download directory for downloaded chapters (CBZ archives and legacy directories)
	downloadedIds := chapter_downloader.ScanDownloadDir(r.downloadDir)

	if len(downloadedIds) > 0 {

		// Now that we have all the downloaded chapters, we can get the chapter containers

		providerAndMediaIdPairs := make(map[struct {
			provider string
			mediaId  int
		}]bool)

		for key := range downloadedIds {
			providerAndMediaIdPairs[struct {
				provider string
				mediaId  int
			}{
				provider: key.Provider,
				mediaId:  key.MediaId,
			}] = true
		}

		// Chapter lookup by identity, ignoring chapter number formatting
		type chapterKey struct {
			provider  string
			mediaId   int
			chapterId string
		}
		downloadedChapterKeys := make(map[chapterKey]bool, len(downloadedIds))
		for key := range downloadedIds {
			downloadedChapterKeys[chapterKey{key.Provider, key.MediaId, key.ChapterId}] = true
		}

		// Get the chapter containers
		for pair := range providerAndMediaIdPairs {
			provider := pair.provider
			mediaId := pair.mediaId

			//// Get the manga from the collection
			//mangaEntry, ok := mangaCollection.GetListEntryFromMangaId(mediaId)
			//if !ok {
			//	r.logger.Warn().Int("mediaId", mediaId).Msg("manga: [GetDownloadedChapterContainers] Manga not found in collection")
			//	continue
			//}

			// Get the list of chapters for the manga
			// Check the permanent file cache
			container, found := r.getChapterContainerFromPermanentFilecache(provider, mediaId)
			if !found {
				// Check the temporary file cache
				container, found = r.getChapterContainerFromFilecache(provider, mediaId)
				if !found {
					continue
					//// Get the chapters from the provider
					//// This stays here for backwards compatibility, but ideally the method should not require an internet connection
					//// so this will fail if the chapters were not cached & with no internet
					//opts := GetMangaChapterContainerOptions{
					//	Provider: provider,
					//	MediaId:  mediaId,
					//	Titles:   mangaEntry.GetMedia().GetAllTitles(),
					//	Year:     mangaEntry.GetMedia().GetStartYearSafe(),
					//}
					//container, err = r.GetMangaChapterContainer(&opts)
					//if err != nil {
					//	r.logger.Error().Err(err).Int("mediaId", mediaId).Msg("manga: [GetDownloadedChapterContainers] Failed to retrieve cached list of manga chapters")
					//	continue
					//}
					//// Cache the chapter container in the permanent bucket
					//go func() {
					//	chapterContainerKey := getMangaChapterContainerCacheKey(provider, mediaId)
					//	chapterContainer, found := r.getChapterContainerFromFilecache(provider, mediaId)
					//	if found {
					//		// Store the chapter container in the permanent bucket
					//		permBucket := getPermanentChapterContainerCacheBucket(provider, mediaId)
					//		_ = r.fileCacher.SetPerm(permBucket, chapterContainerKey, chapterContainer)
					//	}
					//}()
				}
			} else {
				r.logger.Trace().Int("mediaId", mediaId).Msg("manga: Found chapter container in permanent bucket")
			}

			downloadedContainer := &ChapterContainer{
				MediaId:  container.MediaId,
				Provider: container.Provider,
				Chapters: make([]*hibikemanga.ChapterDetails, 0),
			}

			// Now that we have the container, we'll filter out the chapters that are not downloaded
			// Go through each chapter and check if it's downloaded
			for _, chapter := range container.Chapters {
				if downloadedChapterKeys[chapterKey{provider, mediaId, chapter.ID}] {
					downloadedContainer.Chapters = append(downloadedContainer.Chapters, chapter)
				}
			}

			if len(downloadedContainer.Chapters) == 0 {
				continue
			}

			ret = append(ret, downloadedContainer)
		}
	}

	// Add chapter containers from local provider
	localProviderB, ok := extension.GetExtension[extension.MangaProviderExtension](r.extensionBankRef.Get(), manga_providers.LocalProvider)
	if ok {
		_, ok := localProviderB.GetProvider().(*manga_providers.Local)
		if ok {
			for _, list := range mangaCollection.MediaListCollection.GetLists() {
				for _, entry := range list.GetEntries() {
					media := entry.GetMedia()
					opts := GetMangaChapterContainerOptions{
						Provider: manga_providers.LocalProvider,
						MediaId:  media.GetID(),
						Titles:   media.GetAllTitles(),
						Year:     media.GetStartYearSafe(),
					}
					container, err := r.GetMangaChapterContainer(&opts)
					if err != nil {
						continue
					}
					ret = append(ret, container)
				}
			}
		}
	}

	// Event
	ev := &MangaDownloadedChapterContainersEvent{
		ChapterContainers: ret,
	}
	err = hook.GlobalHookManager.OnMangaDownloadedChapterContainers().Trigger(ev)
	if err != nil {
		r.logger.Error().Err(err).Msg("manga: Exception occurred while triggering hook event")
		return nil, fmt.Errorf("manga: Error in hook, %w", err)
	}
	ret = ev.ChapterContainers

	return ret, nil
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// getDownloadedMangaPageContainer retrieves page information for a downloaded manga chapter.
// It locates the chapter's CBZ archive (or legacy loose-image directory) to build a
// PageContainer with details about each downloaded page including dimensions and file paths.
// Page URLs are relative to the download directory, slash-separated, and resolved by the
// /manga-downloads/* route (which streams entries out of CBZ archives).
func (r *Repository) getDownloadedMangaPageContainer(
	provider string,
	mediaId int,
	chapterId string,
) (*PageContainer, error) {

	// Check if the chapter is downloaded
	found := false
	relPath := "" // e.g. comick_123/0013_10010.cbz or legacy comick_123_10010_13/

	for downloadId, p := range chapter_downloader.ScanDownloadDir(r.downloadDir) {
		if downloadId.Provider == provider &&
			downloadId.MediaId == mediaId &&
			downloadId.ChapterId == chapterId {
			found = true
			relPath = p
			break
		}
	}

	if !found {
		return nil, ErrChapterNotDownloaded
	}

	r.logger.Debug().Str("path", relPath).Msg("manga: Found downloaded chapter")

	pageList := make([]*hibikemanga.ChapterPage, 0)
	pageDimensions := make(map[int]*PageDimension)

	if strings.HasSuffix(relPath, ".cbz") {

		entries, _, err := chapter_downloader.ReadCBZ(filepath.Join(r.downloadDir, filepath.FromSlash(relPath)))
		if err != nil {
			r.logger.Error().Err(err).Msg("manga: Failed to read chapter archive")
			return nil, err
		}

		for i, entry := range entries {
			pageList = append(pageList, &hibikemanga.ChapterPage{
				Index:    i,
				URL:      relPath + "/" + entry.Name,
				Provider: provider,
			})
			pageDimensions[i] = &PageDimension{
				Width:  entry.Width,
				Height: entry.Height,
			}
		}

	} else {

		// Legacy loose-image directory: read registry.json
		registryFile, err := os.Open(filepath.Join(r.downloadDir, relPath, "registry.json"))
		if err != nil {
			r.logger.Error().Err(err).Msg("manga: Failed to open registry file")
			return nil, err
		}
		defer registryFile.Close()

		r.logger.Debug().Str("chapterId", chapterId).Msg("manga: Reading registry file")

		var pageRegistry *chapter_downloader.Registry
		err = json.NewDecoder(registryFile).Decode(&pageRegistry)
		if err != nil {
			r.logger.Error().Err(err).Msg("manga: Failed to decode registry file")
			return nil, err
		}

		for pageIndex, pageInfo := range *pageRegistry {
			pageList = append(pageList, &hibikemanga.ChapterPage{
				Index:    pageIndex,
				URL:      relPath + "/" + pageInfo.Filename,
				Provider: provider,
			})
			pageDimensions[pageIndex] = &PageDimension{
				Width:  pageInfo.Width,
				Height: pageInfo.Height,
			}
		}
	}

	slices.SortStableFunc(pageList, func(i, j *hibikemanga.ChapterPage) int {
		return cmp.Compare(i.Index, j.Index)
	})

	container := &PageContainer{
		MediaId:        mediaId,
		Provider:       provider,
		ChapterId:      chapterId,
		Pages:          pageList,
		PageDimensions: pageDimensions,
		IsDownloaded:   true,
	}

	r.logger.Debug().Str("chapterId", chapterId).Msg("manga: Found downloaded chapter")

	return container, nil
}
