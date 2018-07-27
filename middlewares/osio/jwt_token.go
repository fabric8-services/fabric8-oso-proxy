package osio

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/containous/traefik/log"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/pkg/errors"
	"gopkg.in/square/go-jose.v1"
)

type PublicKey struct {
	KeyID string
	Key   *rsa.PublicKey
}

// JSONKeys the remote keys encoded in a json document
type JSONKeys struct {
	Keys []interface{} `json:"keys"`
}

func CreateTokenTypeLocator(client *http.Client, authURL string) TokenTypeLocator {
	var publicKeysMap map[string]*rsa.PublicKey

	keyFunc := func(token *jwt.Token) (interface{}, error) {
		kid := token.Header["kid"]
		if kid == nil {
			log.Error("There is no 'kid' header in the token")
			return nil, errors.New("There is no 'kid' header in the token")
		}
		key := publicKeysMap[fmt.Sprintf("%s", kid)]
		if key == nil {
			log.Error("There is no public key with such ID")
			return nil, errors.New(fmt.Sprintf("There is no public key with such ID: %s", kid))
		}
		return key, nil
	}

	return func(token string) (TokenType, error) {
		if publicKeysMap == nil {
			remoteKeys, err := fetchKeys(client, authURL)
			if err != nil {
				return "", err
			}
			publicKeysMap = make(map[string]*rsa.PublicKey)
			for _, remoteKey := range remoteKeys {
				publicKeysMap[remoteKey.KeyID] = remoteKey.Key
			}
		}
		jwtToken, err := jwt.Parse(token, keyFunc)
		if err != nil {
			return "", err
		}
		accountName := jwtToken.Claims.(jwt.MapClaims)["service_accountname"]
		if accountName != nil {
			accNameStr, isString := accountName.(string)
			if isString {
				tokenType := TokenTypeMap[accNameStr]
				if tokenType == "" {
					return "", fmt.Errorf("service_accountname '%s' not supported", accNameStr)
				}
				return tokenType, nil
			}
			return "", fmt.Errorf("Not valid JWT token")
		}
		sub := jwtToken.Claims.(jwt.MapClaims)["sub"]
		if sub != nil {
			_, isString := sub.(string)
			if isString {
				return UserToken, nil
			}
			return "", fmt.Errorf("Not valid JWT token")
		}
		return "", fmt.Errorf("Not valid JWT token")
	}
}

func fetchKeys(client *http.Client, authURL string) ([]*PublicKey, error) {
	keysEndpointURL := authURL + "/token/keys"
	req, err := http.NewRequest("GET", keysEndpointURL, nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Failed to obtain public keys from remote service, call to '%s' failed with status '%s'", keysEndpointURL, res.Status)
	}
	keys, err := unmarshalKeys(body)
	if err != nil {
		return nil, err
	}
	return keys, nil
}

func unmarshalKeys(jsonData []byte) ([]*PublicKey, error) {
	var keys []*PublicKey
	var raw JSONKeys
	err := json.Unmarshal(jsonData, &raw)
	if err != nil {
		return nil, err
	}
	for _, key := range raw.Keys {
		jsonKeyData, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		publicKey, err := unmarshalKey(jsonKeyData)
		if err != nil {
			return nil, err
		}
		keys = append(keys, publicKey)
	}
	return keys, nil
}

func unmarshalKey(jsonData []byte) (*PublicKey, error) {
	var key *jose.JsonWebKey
	key = &jose.JsonWebKey{}
	err := key.UnmarshalJSON(jsonData)
	if err != nil {
		return nil, err
	}
	rsaKey, ok := key.Key.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("Key is not an *rsa.PublicKey")
	}
	return &PublicKey{key.KeyID, rsaKey}, nil
}
