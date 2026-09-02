package wechat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Client struct {
	appID    string
	secret   string
	endpoint string
	baseURL  string
	http     *http.Client
	mu       sync.Mutex
	token    string
	tokenExp time.Time
}

func NewClient(appID, secret, endpoint string, timeout time.Duration) (*Client, error) {
	if strings.TrimSpace(appID) == "" || strings.TrimSpace(secret) == "" {
		return nil, fmt.Errorf("wechat app id and app secret are required")
	}
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(endpoint)), "https://") {
		return nil, fmt.Errorf("wechat endpoint must use https")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("wechat endpoint is invalid")
	}
	return &Client{appID: appID, secret: secret, endpoint: endpoint, baseURL: parsed.Scheme + "://" + parsed.Host, http: &http.Client{Timeout: timeout}}, nil
}

type codeSessionResponse struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

func (c *Client) ExchangeCode(ctx context.Context, code string) (string, error) {
	query := url.Values{}
	query.Set("appid", c.appID)
	query.Set("secret", c.secret)
	query.Set("js_code", strings.TrimSpace(code))
	query.Set("grant_type", "authorization_code")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint+"?"+query.Encode(), nil)
	if err != nil {
		return "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("wechat code2session request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("wechat code2session returned http %d", resp.StatusCode)
	}
	var result codeSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode wechat code2session response: %w", err)
	}
	if result.ErrCode != 0 || strings.TrimSpace(result.OpenID) == "" {
		return "", fmt.Errorf("wechat code2session failed: %d %s", result.ErrCode, result.ErrMsg)
	}
	return result.OpenID, nil
}

type SubscribeMessageParams struct {
	ToUser     string
	TemplateID string
	Page       string
	Data       map[string]string
}

type subscribeDataValue struct {
	Value string `json:"value"`
}

type subscribeMessageRequest struct {
	ToUser           string                        `json:"touser"`
	TemplateID       string                        `json:"template_id"`
	Page             string                        `json:"page,omitempty"`
	MiniprogramState string                        `json:"miniprogram_state,omitempty"`
	Data             map[string]subscribeDataValue `json:"data"`
}

type accessTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

type subscribeMessageResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func (c *Client) SendSubscribeMessage(ctx context.Context, params SubscribeMessageParams) error {
	accessToken, err := c.accessToken(ctx)
	if err != nil {
		return err
	}
	data := make(map[string]subscribeDataValue, len(params.Data))
	for key, value := range params.Data {
		if strings.TrimSpace(key) != "" && strings.TrimSpace(value) != "" {
			data[key] = subscribeDataValue{Value: strings.TrimSpace(value)}
		}
	}
	body, err := json.Marshal(subscribeMessageRequest{ToUser: strings.TrimSpace(params.ToUser), TemplateID: strings.TrimSpace(params.TemplateID), Page: strings.TrimSpace(params.Page), MiniprogramState: "formal", Data: data})
	if err != nil {
		return fmt.Errorf("encode wechat subscribe message: %w", err)
	}
	requestURL := c.baseURL + "/cgi-bin/message/subscribe/send?access_token=" + url.QueryEscape(accessToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("wechat subscribe message request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("wechat subscribe message returned http %d", resp.StatusCode)
	}
	var result subscribeMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode wechat subscribe message response: %w", err)
	}
	if result.ErrCode != 0 {
		return fmt.Errorf("wechat subscribe message failed: %d %s", result.ErrCode, result.ErrMsg)
	}
	return nil
}

func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	if c.token != "" && time.Now().Before(c.tokenExp) {
		token := c.token
		c.mu.Unlock()
		return token, nil
	}
	c.mu.Unlock()

	query := url.Values{"grant_type": {"client_credential"}, "appid": {c.appID}, "secret": {c.secret}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/cgi-bin/token?"+query.Encode(), nil)
	if err != nil {
		return "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("wechat access token request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("wechat access token returned http %d", resp.StatusCode)
	}
	var result accessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode wechat access token response: %w", err)
	}
	if result.ErrCode != 0 || strings.TrimSpace(result.AccessToken) == "" {
		return "", fmt.Errorf("wechat access token failed: %d %s", result.ErrCode, result.ErrMsg)
	}
	expiresIn := time.Duration(result.ExpiresIn) * time.Second
	if expiresIn <= 0 {
		expiresIn = 2 * time.Hour
	}
	c.mu.Lock()
	c.token = result.AccessToken
	c.tokenExp = time.Now().Add(expiresIn - time.Minute)
	c.mu.Unlock()
	return result.AccessToken, nil
}
