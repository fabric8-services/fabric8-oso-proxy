package osio

import (
	"fmt"
	"net/http"
)

type OSIORequest struct {
}

func NewOSIORequest() *OSIORequest {
	return &OSIORequest{}
}

// ServeHTTP handle OSIORequest middleware. It moves access_token from query param to request header.
func (a *OSIORequest) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	if r.Method != "OPTIONS" {
		processAuthParam(r)
	}
	next(rw, r)
}

func processAuthParam(r *http.Request) {
	accessToken := r.URL.Query().Get("access_token")
	if accessToken != "" {
		addAuthHeader(r, accessToken)
		removeAuthParam(r)
	}
}

func addAuthHeader(r *http.Request, accessToken string) {
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
}

func removeAuthParam(r *http.Request) {
	q := r.URL.Query()
	q.Del("access_token")
	r.URL.RawQuery = q.Encode()
	r.RequestURI = r.URL.RequestURI()
}
