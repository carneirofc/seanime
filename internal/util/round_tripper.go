package util

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// Full credit to https://github.com/DaRealFreak/cloudflare-bp-go

// RetryConfig configures the retry behavior
type RetryConfig struct {
	MaxRetries    int
	RetryDelay    time.Duration
	TimeoutOnly   bool  // Only retry on timeout errors
	RetryOnStatus []int // Also retry on these HTTP status codes (e.g., 429, 503)
}

// cloudFlareRoundTripper is a custom round tripper add the validated request headers.
type cloudFlareRoundTripper struct {
	inner   http.RoundTripper
	options Options
	retry   *RetryConfig
}

// Options the option to set custom headers
type Options struct {
	AddMissingHeaders bool
	Headers           map[string]string
}

// AddCloudFlareByPass returns a round tripper adding the required headers for the CloudFlare checks
// and updates the TLS configuration of the passed inner transport.
func AddCloudFlareByPass(inner http.RoundTripper, options ...Options) http.RoundTripper {
	if trans, ok := inner.(*http.Transport); ok {
		trans.TLSClientConfig = getCloudFlareTLSConfiguration()
	}

	roundTripper := &cloudFlareRoundTripper{
		inner: inner,
		retry: &RetryConfig{
			MaxRetries:    3,
			RetryDelay:    2 * time.Second,
			TimeoutOnly:   false,
			RetryOnStatus: []int{429, 503},
		},
	}

	if len(options) > 0 {
		roundTripper.options = options[0]
	} else {
		roundTripper.options = GetDefaultOptions()
	}

	return roundTripper
}

// RoundTrip adds the required request headers to pass CloudFlare checks.
func (ug *cloudFlareRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	var lastErr error
	attempts := 0

	for attempts <= ug.retry.MaxRetries {
		// Add headers for this attempt
		if ug.options.AddMissingHeaders {
			for header, value := range ug.options.Headers {
				if _, ok := r.Header[header]; !ok {
					if header == "User-Agent" {
						r.Header.Set(header, "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
					} else {
						r.Header.Set(header, value)
					}
				}
			}
		}

		// Make the request
		var resp *http.Response
		var err error

		// in case we don't have an inner transport layer from the round tripper
		if ug.inner == nil {
			resp, err = (&http.Transport{
				TLSClientConfig:   getCloudFlareTLSConfiguration(),
				ForceAttemptHTTP2: false,
			}).RoundTrip(r)
		} else {
			resp, err = ug.inner.RoundTrip(r)
		}

		// Check if we should retry
		shouldRetry := false
		if err != nil {
			if ug.retry.TimeoutOnly {
				shouldRetry = errors.Is(err, http.ErrHandlerTimeout)
			} else {
				shouldRetry = true
			}
		} else if resp != nil && len(ug.retry.RetryOnStatus) > 0 {
			for _, code := range ug.retry.RetryOnStatus {
				if resp.StatusCode == code {
					shouldRetry = true
					break
				}
			}
		}

		// If successful and no retry needed, return immediately
		if !shouldRetry {
			return resp, err
		}

		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		attempts++

		// If we have more retries, wait before next attempt (exponential backoff for status retries)
		if attempts <= ug.retry.MaxRetries {
			delay := ug.retry.RetryDelay * time.Duration(attempts)
			time.Sleep(delay)
		}
	}

	return nil, lastErr
}

// getCloudFlareTLSConfiguration returns an accepted client TLS configuration to not get detected by CloudFlare directly
// in case the configuration needs to be updated later on: https://wiki.mozilla.org/Security/Server_Side_TLS .
func getCloudFlareTLSConfiguration() *tls.Config {
	return &tls.Config{
		CurvePreferences: []tls.CurveID{tls.CurveP256, tls.CurveP384, tls.CurveP521, tls.X25519},
	}
}

// GetDefaultOptions returns the options set by default
func GetDefaultOptions() Options {
	return Options{
		AddMissingHeaders: true,
		Headers: map[string]string{
			"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8",
			"Accept-Language": "en-US,en;q=0.5",
			"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
		},
	}
}
