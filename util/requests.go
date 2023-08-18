package util

import (
	"fmt"
	"net/http"
)

const (
	dearrowApiURL = "https://sponsor.ajay.app/api/branding?videoID=%s&returnUserID=%t"
)

func FetchVideoBranding(client *http.Client, videoID string, returnUserID bool) (*http.Response, error) {
	return client.Get(fmt.Sprintf(dearrowApiURL, videoID, returnUserID))
}
