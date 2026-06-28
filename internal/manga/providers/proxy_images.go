package manga_providers

import (
	"fmt"
	util "seanime/internal/util/proxies"
	"time"
)

func GetImageByProxy(url string, headers map[string]string) ([]byte, error) {
	ip := &util.ImageProxy{}
	image, _, err := ip.GetImage(url, headers)
	return image, err
}

// GetImageByProxyWithRetry fetches an image through the proxy, retrying transient
// failures (network errors, non-2xx responses, empty bodies) with a linear backoff.
// maxRetries is the number of additional attempts after the first one.
func GetImageByProxyWithRetry(url string, headers map[string]string, maxRetries int) (buf []byte, err error) {
	for attempt := 0; attempt <= maxRetries; attempt++ {
		buf, err = GetImageByProxy(url, headers)
		if err == nil && len(buf) > 0 {
			return buf, nil
		}
		if err == nil {
			err = fmt.Errorf("empty image response from %s", url)
		}
		if attempt < maxRetries {
			time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
		}
	}
	return nil, err
}
