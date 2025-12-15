package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type KeycloakClient struct {
	baseURL      string
	clientID     string
	clientSecret string
	username     string
	password     string
	httpClient   *http.Client
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

func NewKeycloakClient(baseURL, clientID, clientSecret, username, password string) *KeycloakClient {
	return &KeycloakClient{
		baseURL:      baseURL,
		clientID:     clientID,
		clientSecret: clientSecret,
		username:     username,
		password:     password,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (k *KeycloakClient) GetAccessToken() (string, error) {
	tokenURL := fmt.Sprintf("%s/realms/IPN/protocol/openid-connect/token", k.baseURL)

	data := url.Values{}
	data.Set("grant_type", "password")
	data.Set("client_id", k.clientID)
	data.Set("client_secret", k.clientSecret)
	data.Set("username", k.username)
	data.Set("password", k.password)

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("authentication failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	return tokenResp.AccessToken, nil
}

func (k *KeycloakClient) RefreshAccessToken(refreshToken string) (string, error) {
	tokenURL := fmt.Sprintf("%s/realms/IPN/protocol/openid-connect/token", k.baseURL)

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("client_id", k.clientID)
	data.Set("client_secret", k.clientSecret)
	data.Set("refresh_token", refreshToken)

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create refresh request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make refresh request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode refresh token response: %w", err)
	}

	return tokenResp.AccessToken, nil
}
