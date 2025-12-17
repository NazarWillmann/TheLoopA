package asterisk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

type ARIClient struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
	logger     zerolog.Logger
}

type OriginateRequest struct {
	Endpoint       string            `json:"endpoint"`
	Extension      string            `json:"extension,omitempty"`
	Context        string            `json:"context,omitempty"`
	Priority       int               `json:"priority,omitempty"`
	App            string            `json:"app,omitempty"`
	AppArgs        []string          `json:"appArgs,omitempty"`
	CallerID       string            `json:"callerId,omitempty"`
	Timeout        int               `json:"timeout,omitempty"`
	Variables      map[string]string `json:"variables,omitempty"`
	ChannelID      string            `json:"channelId,omitempty"`
	OtherChannelID string            `json:"otherChannelId,omitempty"`
}

type OriginateResponse struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	State        string            `json:"state"`
	CallerID     CallerID          `json:"caller"`
	Connected    CallerID          `json:"connected"`
	AccountCode  string            `json:"accountcode"`
	Dialplan     DialplanCEP       `json:"dialplan"`
	CreationTime string            `json:"creationtime"`
	Language     string            `json:"language"`
	Variables    map[string]string `json:"channelvars,omitempty"`
}

type CallerID struct {
	Name   string `json:"name"`
	Number string `json:"number"`
}

type DialplanCEP struct {
	Context   string `json:"context"`
	Extension string `json:"exten"`
	Priority  int    `json:"priority"`
}

type Channel struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	State        string            `json:"state"`
	CallerID     CallerID          `json:"caller"`
	Connected    CallerID          `json:"connected"`
	AccountCode  string            `json:"accountcode"`
	Dialplan     DialplanCEP       `json:"dialplan"`
	CreationTime string            `json:"creationtime"`
	Language     string            `json:"language"`
	Variables    map[string]string `json:"channelvars,omitempty"`
}

type ARIError struct {
	Message string `json:"message"`
}

func NewARIClient(baseURL, username, password string, logger zerolog.Logger) *ARIClient {
	return &ARIClient{
		baseURL:  strings.TrimSuffix(baseURL, "/"),
		username: username,
		password: password,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

func (c *ARIClient) Originate(endpoint, callerID string, variables map[string]string) (string, error) {
	c.logger.Debug().
		Str("endpoint", endpoint).
		Str("caller_id", callerID).
		Interface("variables", variables).
		Msg("Originating call via ARI")

	request := OriginateRequest{
		Endpoint:  endpoint,
		CallerID:  callerID,
		Timeout:   30,
		Variables: variables,
		App:       "Stasis",
		AppArgs:   []string{"call-emulator"},
	}

	data, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal originate request: %w", err)
	}

	requestURL := fmt.Sprintf("%s/channels", c.baseURL)
	req, err := http.NewRequest("POST", requestURL, bytes.NewBuffer(data))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.username, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var ariError ARIError
		if err := json.Unmarshal(body, &ariError); err == nil {
			return "", fmt.Errorf("ARI error (%d): %s", resp.StatusCode, ariError.Message)
		}
		return "", fmt.Errorf("ARI request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response OriginateResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to unmarshal originate response: %w", err)
	}

	c.logger.Info().
		Str("channel_id", response.ID).
		Str("channel_name", response.Name).
		Str("state", response.State).
		Msg("Call originated successfully")

	return response.ID, nil
}

func (c *ARIClient) GetChannel(channelID string) (*Channel, error) {
	requestURL := fmt.Sprintf("%s/channels/%s", c.baseURL, channelID)
	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("channel not found")
	}

	if resp.StatusCode != http.StatusOK {
		var ariError ARIError
		if err := json.Unmarshal(body, &ariError); err == nil {
			return nil, fmt.Errorf("ARI error (%d): %s", resp.StatusCode, ariError.Message)
		}
		return nil, fmt.Errorf("ARI request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var channel Channel
	if err := json.Unmarshal(body, &channel); err != nil {
		return nil, fmt.Errorf("failed to unmarshal channel response: %w", err)
	}

	return &channel, nil
}

func (c *ARIClient) HangupChannel(channelID string) error {
	requestURL := fmt.Sprintf("%s/channels/%s", c.baseURL, channelID)
	req, err := http.NewRequest("DELETE", requestURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		c.logger.Warn().Str("channel_id", channelID).Msg("Channel not found for hangup")
		return nil
	}

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("hangup failed with status %d: %s", resp.StatusCode, string(body))
	}

	c.logger.Info().Str("channel_id", channelID).Msg("Channel hung up successfully")
	return nil
}

func (c *ARIClient) AnswerChannel(channelID string) error {
	requestURL := fmt.Sprintf("%s/channels/%s/answer", c.baseURL, channelID)
	req, err := http.NewRequest("POST", requestURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		var err = Body.Close()
		if err != nil {

		}
	}(resp.Body)

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("channel not found")
	}

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("answer failed with status %d: %s", resp.StatusCode, string(body))
	}

	c.logger.Info().Str("channel_id", channelID).Msg("Channel answered successfully")
	return nil
}

func (c *ARIClient) SetChannelVariable(channelID, variable, value string) error {
	requestURL := fmt.Sprintf("%s/channels/%s/variable", c.baseURL, channelID)

	params := url.Values{}
	params.Set("variable", variable)
	params.Set("value", value)

	req, err := http.NewRequest("POST", requestURL, strings.NewReader(params.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(c.username, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("channel not found")
	}

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("set variable failed with status %d: %s", resp.StatusCode, string(body))
	}

	c.logger.Debug().
		Str("channel_id", channelID).
		Str("variable", variable).
		Str("value", value).
		Msg("Channel variable set successfully")

	return nil
}

func (c *ARIClient) GetChannelVariable(channelID, variable string) (string, error) {
	requestURL := fmt.Sprintf("%s/channels/%s/variable?variable=%s", c.baseURL, channelID, variable)
	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		var err = Body.Close()
		if err != nil {

		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("channel or variable not found")
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("get variable failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to unmarshal variable response: %w", err)
	}

	return result.Value, nil
}
