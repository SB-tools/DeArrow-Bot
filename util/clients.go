package util

import (
	"net/http"
	"os"
	"time"
)

var (
	priorityKey = os.Getenv("DEARROW_PRIORITY_KEY")
)

const (
	brandingTimeout  = 2 * time.Second // this is quite ambitious
	thumbnailTimeout = 30 * time.Second
)

func NewBrandingClient() *http.Client {
	return &http.Client{
		Timeout: brandingTimeout,
	}
}

func NewThumbnailClient() *http.Client {
	return &http.Client{
		Timeout:   thumbnailTimeout,
		Transport: &thumbnailTripper{tripper: http.DefaultTransport},
	}
}

type thumbnailTripper struct {
	tripper http.RoundTripper
}

func (t *thumbnailTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("Authorization", priorityKey)
	return t.tripper.RoundTrip(req)
}
