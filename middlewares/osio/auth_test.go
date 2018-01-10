package middlewares

import (
	"fmt"
	"testing"
)

func TestAuthShouldX(t *testing.T) {
	witURL := "https://api.prod-preview.openshift.io/api"
	authURL := "https://auth.prod-preview.openshift.io/api"

	a := NewOSIOAuth(witURL, authURL)
	fmt.Println(a)
}
