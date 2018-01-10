package middlewares

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func CreateTenantTokenLocator(client *http.Client, authBaseURL string) TenantTokenLocator {
	return func(token, location string) (string, error) {
		return locateToken(client, authBaseURL, token, location)
	}
}

type token struct {
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	Type        string `json:"token_type"`
}

func locateToken(client *http.Client, authBaseURL, osioToken, location string) (string, error) {

	req, err := http.NewRequest("GET", authBaseURL+"/token?for="+location, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set(Authorization, "Bearer "+osioToken)

	resp, err := client.Do(req)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Unknown status code " + resp.Status)
	}
	defer resp.Body.Close()

	var t token
	json.NewDecoder(resp.Body).Decode(&t)
	return t.AccessToken, nil
}
