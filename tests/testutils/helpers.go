package testutils

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"time"
)

// GetAppURL returns the application URL for testing
func GetAppURL() string {
	if url := os.Getenv("APP_URL"); url != "" {
		return url
	}
	if url := os.Getenv("BASE_URL"); url != "" {
		return url
	}
	return "http://localhost:8080"
}

// WaitForServices waits for the application and dependencies to be ready
func WaitForServices(baseURL string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for services to be ready")
		case <-ticker.C:
			// Check application health
			resp, err := http.Get(baseURL + "/health")
			if err == nil && resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				return nil
			}
			if resp != nil {
				resp.Body.Close()
			}
		}
	}
}

// MakeRequest makes an HTTP request with optional authentication
func MakeRequest(method, url string, headers map[string]string, body []byte) (*http.Response, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	var req *http.Request
	var err error

	if body != nil {
		req, err = http.NewRequest(method, url, bytes.NewBuffer(body))
	} else {
		req, err = http.NewRequest(method, url, nil)
	}
	if err != nil {
		return nil, err
	}

	// Add headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return client.Do(req)
}
