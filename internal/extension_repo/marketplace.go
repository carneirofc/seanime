package extension_repo

import (
	"context"
	"fmt"
	"seanime/internal/constants"
	"seanime/internal/extension"
	"seanime/internal/util"

	"github.com/goccy/go-json"
	"github.com/samber/lo"
)

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

	err = json.Unmarshal(bodyR, &extensions)
	if err != nil {
		r.logger.Error().Err(err).Msgf("marketplace: Failed to unmarshal marketplace extension: %s", url)
		return nil, fmt.Errorf("failed to unmarshal marketplace extension: %s", url)
	}

	extensions = lo.Filter(extensions, func(item *extension.Extension, _ int) bool {
		return item.ID != "" && item.ManifestURI != ""
	})

	return
}
