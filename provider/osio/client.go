package osio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type Client interface {
	CallTokenAPI(tokenAPI string, tokenReq *TokenRequest) (*TokenResponse, error)
	CallClusterAPI(clusterAPIURL string, tokenResp *TokenResponse) (*clusterResponse, error)
}

func NewClient() Client {
	return &authClient{Client: http.DefaultClient}
}

type authClient struct {
	*http.Client
}

type TokenRequest struct {
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

type clusterData struct {
	APIURL     string `json:"api-url"`
	AppDNS     string `json:"app-dns"`
	ConsoleURL string `json:"console-url"`
	LoggingURL string `json:"logging-url"`
	MetricsURL string `json:"metrics-url"`
	Name       string `json:"name"`
}

type clusterResponse struct {
	Clusters []clusterData `json:"data"`
}

func (client *authClient) CallTokenAPI(tokenAPI string, tokenReq *TokenRequest) (*TokenResponse, error) {
	reqBody := new(bytes.Buffer)
	err := json.NewEncoder(reqBody).Encode(tokenReq)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, tokenAPI, reqBody)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Call to token api failed, code:%d, error:%s", resp.StatusCode, resp.Status)
	}

	defer resp.Body.Close()
	var tokenResp *TokenResponse
	err = json.NewDecoder(resp.Body).Decode(&tokenResp)
	if err != nil {
		return nil, err
	}
	return tokenResp, nil
}

func (client *authClient) CallClusterAPI(clusterAPIURL string, tokenResp *TokenResponse) (*clusterResponse, error) {
	req, err := http.NewRequest(http.MethodGet, clusterAPIURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(authorization, fmt.Sprintf("%s %s", tokenResp.TokenType, tokenResp.AccessToken))
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Cluster API call failed with code:%d, error:%s", resp.StatusCode, resp.Status)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var clusters = new(clusterResponse)
	err = json.Unmarshal(body, &clusters)
	if err != nil {
		return nil, err
	}
	return clusters, nil
}
