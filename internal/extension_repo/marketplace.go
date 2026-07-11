package extension_repo

import (
	"context"
	"fmt"
	"io"
	"net/http"
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
	req, err := newExtensionRequest(context.Background(), url)
	if err != nil {
		r.logger.Error().Err(err).Msgf("marketplace: Failed to create request for marketplace extension: %s", url)
		return nil, fmt.Errorf("failed to get marketplace extension: %s", url)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		r.logger.Error().Err(err).Msgf("marketplace: Failed to get marketplace extension: %s", url)
		return nil, fmt.Errorf("failed to get marketplace extension: %s", url)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		r.logger.Error().Int("status", resp.StatusCode).Msgf("marketplace: Failed to get marketplace extension: %s", url)
		hint := ""
		if isGitHubHost(req.URL.Hostname()) && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound) {
			hint = ", private GitHub repositories require a token"
		}
		return nil, fmt.Errorf("failed to get marketplace extension (status %d)%s: %s", resp.StatusCode, hint, url)
	}

	bodyR, err := io.ReadAll(resp.Body)
	if err != nil {
		r.logger.Error().Err(err).Msgf("marketplace: Failed to read marketplace extension: %s", url)
		return nil, fmt.Errorf("failed to read marketplace extension: %s", url)
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
