package middlewares

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/containous/traefik/provider/osio"
	"golang.org/x/crypto/openpgp"
)

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	Type        string `json:"token_type"`
}

type tenantTokenLocator struct {
	client      *http.Client
	authBaseURL string
}

func (t *tenantTokenLocator) GetTokenWithUserToken(userToken, location string) (string, error) {
	return locateToken(t.client, t.authBaseURL, userToken, location)
}

func (t *tenantTokenLocator) GetTokenWithSAToken(saToken, location string) (string, error) {
	encryptedClusterToken, err := locateToken(t.client, t.authBaseURL, saToken, location)
	if err != nil {
		return "", err
	}
	passphrase := os.Getenv("AUTH_TOKEN_KEY")
	clusterToken, err := gpgDecyptToken(encryptedClusterToken, passphrase)
	if err != nil {
		return "", err
	}
	return clusterToken, nil
}

func CreateSrvAccTokenLocator(authBaseURL, srvAccID, srvAccSecret string) SrvAccTokenLocator {
	client := osio.NewClient()
	tokenReq := &osio.TokenRequest{GrantType: "client_credentials", ClientID: srvAccID, ClientSecret: srvAccSecret}
	saToken := ""
	return func() (string, error) {
		if saToken != "" {
			return saToken, nil
		}
		res, err := client.CallTokenAPI(authBaseURL+"/token", tokenReq)
		if err != nil {
			return "", err
		}
		saToken = res.AccessToken
		return saToken, nil
	}
}

func CreateTenantTokenLocator(client *http.Client, authBaseURL string) TenantTokenLocator {
	return &tenantTokenLocator{client: client, authBaseURL: authBaseURL}
}

func locateToken(client *http.Client, authBaseURL, token, location string) (string, error) {
	req, err := http.NewRequest("GET", authBaseURL+"/token?for="+location, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set(Authorization, "Bearer "+token)

	resp, err := client.Do(req)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Unknown status code " + resp.Status)
	}
	defer resp.Body.Close()

	var t tokenResponse
	err = json.NewDecoder(resp.Body).Decode(&t)
	if err != nil {
		return "", err
	}
	return t.AccessToken, nil
}

func gpgDecyptToken(base64Body, passphrase string) (string, error) {
	decodedEnc, err := base64.StdEncoding.DecodeString(base64Body)
	if err != nil {
		return "", err
	}
	decbuf := bytes.NewBuffer(decodedEnc)
	firstCall := true
	md, err := openpgp.ReadMessage(decbuf, nil, func(keys []openpgp.Key, symmetric bool) ([]byte, error) {
		if firstCall {
			firstCall = false
			return []byte(passphrase), nil
		}
		return nil, errors.New("unable to decrypt token with given key")

	}, nil)
	if err != nil {
		return "", err
	}
	bytes, err := ioutil.ReadAll(md.UnverifiedBody)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
