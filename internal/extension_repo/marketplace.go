package extension_repo

import (
	"context"
	"fmt"
	"seanime/internal/constants"
	"seanime/internal/extension"
	"seanime/internal/util"
	"sync"

	"github.com/goccy/go-json"
	"github.com/samber/lo"
)

// marketplaceHydrationConcurrency bounds the manifest fetches issued to complete entries a
// listing left incomplete, so a large remote marketplace doesn't fan out unbounded.
const marketplaceHydrationConcurrency = 8

func (r *Repository) GetMarketplaceExtensions(url string) (extensions []*extension.Extension, err error) {
	defer util.HandlePanicInModuleWithError("extension_repo/GetMarketplaceExtensions", &err)

	marketplaceUrl := constants.DefaultExtensionMarketplaceURL
	if url != "" {
		marketplaceUrl = url
	}

	return r.getMarketplaceExtensions(marketplaceUrl)
}

func (r *Repository) getMarketplaceExtensions(url string) (extensions []*extension.Extension, err error) {
	bodyR, err := r.fetchExtensionBytes(context.Background(), url)
	if err != nil {
		r.logger.Error().Err(err).Msgf("marketplace: Failed to get marketplace extension: %s", url)
		return nil, fmt.Errorf("failed to get marketplace extension: %s: %w", url, err)
	}

	var entries []*extension.Extension
	if err = json.Unmarshal(bodyR, &entries); err != nil {
		r.logger.Error().Err(err).Msgf("marketplace: Failed to unmarshal marketplace extension: %s", url)
		return nil, fmt.Errorf("failed to unmarshal marketplace extension: %s", url)
	}

	// Resolve every URI an entry declares against the marketplace's own location, then drop
	// the entries that can't be acted on at all, before spending a fetch on any of them.
	candidates := make([]*extension.Extension, 0, len(entries))
	for _, item := range entries {
		// A "null" element in the listing; dereferencing it would panic.
		if item == nil {
			continue
		}

		// A relative manifest URI lets a marketplace list sibling manifests by path, so a
		// monorepo can host the listing alongside the extensions it points at.
		item.ManifestURI = resolveExtensionURI(url, item.ManifestURI)
		item.Icon = resolveMarketplaceAsset(url, item.Icon)
		item.Readme = resolveMarketplaceAsset(url, item.Readme)
		item.Website = resolveMarketplaceAsset(url, item.Website)

		if item.ManifestURI == "" {
			continue
		}

		// The ID becomes a filename under the extension directory on install.
		if idErr := isValidExtensionID(item.ID); idErr != nil {
			r.logger.Warn().Err(idErr).Str("id", item.ID).Msgf("marketplace: Skipping entry with an unusable ID: %s", url)
			continue
		}

		candidates = append(candidates, item)
	}

	r.hydrateMarketplaceEntries(candidates)

	extensions = make([]*extension.Extension, 0, len(candidates))
	for _, item := range candidates {
		if checkErr := marketplaceEntrySanityCheck(item); checkErr != nil {
			r.logger.Warn().Err(checkErr).Str("id", item.ID).Str("uri", item.ManifestURI).Msg("marketplace: Skipping invalid entry")
			continue
		}
		item.Lang = extension.GetExtensionLang(item.Lang)
		extensions = append(extensions, item)
	}

	return extensions, nil
}

// hydrateMarketplaceEntries fills in the display metadata a listing left out by reading each
// entry's own manifest. A listing points at manifests rather than repeating them, so a
// minimal one carries little more than an ID and a manifest URI — and an entry with no type
// is grouped nowhere by the UI and renders as nothing at all. Values the listing did provide
// win, so a marketplace can still present an extension under its own name or icon.
func (r *Repository) hydrateMarketplaceEntries(entries []*extension.Extension) {
	incomplete := lo.Filter(entries, func(item *extension.Extension, _ int) bool {
		return marketplaceEntryNeedsHydration(item)
	})
	if len(incomplete) == 0 {
		return
	}

	sem := make(chan struct{}, marketplaceHydrationConcurrency)
	wg := sync.WaitGroup{}
	wg.Add(len(incomplete))

	for _, item := range incomplete {
		go func(item *extension.Extension) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			// The listing only needs metadata, so skip the payload download; installing
			// fetches the manifest again anyway.
			manifest, fetchErr := r.fetchExternalExtensionData(item.ManifestURI, true)
			if fetchErr != nil {
				r.logger.Warn().Err(fetchErr).Str("id", item.ID).Str("uri", item.ManifestURI).Msg("marketplace: Failed to read entry manifest")
				return
			}

			fillMarketplaceEntry(item, manifest)
		}(item)
	}

	wg.Wait()
}

// marketplaceEntryNeedsHydration reports whether a listing entry is missing any of the
// fields marketplaceEntrySanityCheck requires, or the author shown on its card.
func marketplaceEntryNeedsHydration(ext *extension.Extension) bool {
	return ext.Name == "" || ext.Author == "" || ext.Type == "" || ext.Language == ""
}

// fillMarketplaceEntry copies manifest values into the fields the listing entry left empty.
func fillMarketplaceEntry(entry *extension.Extension, manifest *extension.Extension) {
	fill := func(dst *string, src string) {
		if *dst == "" {
			*dst = src
		}
	}

	fill(&entry.Name, manifest.Name)
	fill(&entry.Version, manifest.Version)
	fill(&entry.Description, manifest.Description)
	fill(&entry.Author, manifest.Author)
	fill(&entry.Notes, manifest.Notes)
	fill(&entry.Lang, manifest.Lang)
	fill(&entry.SemverConstraint, manifest.SemverConstraint)

	if entry.Type == "" {
		entry.Type = manifest.Type
	}
	if entry.Language == "" {
		entry.Language = manifest.Language
	}
	if entry.Plugin == nil {
		entry.Plugin = manifest.Plugin
	}

	// The manifest's own browser-facing URLs are relative to the manifest, not to the
	// marketplace, so they resolve against where the manifest was fetched from.
	if entry.Icon == "" {
		entry.Icon = resolveMarketplaceAsset(manifest.ManifestURI, manifest.Icon)
	}
	if entry.Readme == "" {
		entry.Readme = resolveMarketplaceAsset(manifest.ManifestURI, manifest.Readme)
	}
	if entry.Website == "" {
		entry.Website = resolveMarketplaceAsset(manifest.ManifestURI, manifest.Website)
	}
}

// resolveMarketplaceAsset resolves a browser-facing URL (icon, readme, website) declared
// relative to base. Unlike a manifest or payload URI, which the server fetches, these are
// loaded by the browser: a relative value only works when base is remote, where it resolves
// to a URL the browser can reach too. Against a local base it would resolve to a path on the
// server's filesystem and render as a broken image or a dead link, so it is dropped instead.
func resolveMarketplaceAsset(base, ref string) string {
	if !isRelativeURI(ref) {
		return ref
	}
	if !isRemoteURI(base) {
		return ""
	}
	return resolveExtensionURI(base, ref)
}
