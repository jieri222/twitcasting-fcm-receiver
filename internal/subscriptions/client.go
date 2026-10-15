package subscriptions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"twitcasting-fcm-receiver/internal/client"
	"twitcasting-fcm-receiver/internal/log"
)

const apiBaseURL = "https://frontendapi.twitcasting.tv"

type subscriptionPayload struct {
	SessionID string `json:"cs_session_id"`
	DeviceID  string `json:"device_id"`
	User      string `json:"user"`
}

// Client manages the current account's TwitCasting subscriptions.
type Client struct {
	httpClient *client.Client
	headers    http.Header
	deviceID   string
	sessionID  string
}

// NewClient creates a subscription client from the TC_COOKIE environment variable.
func NewClient() Client {
	defaultHeaders := http.Header{}
	defaultHeaders.Set("Accept", "application/json")
	defaultHeaders.Set("Content-Type", "application/json")
	defaultHeaders.Set("Origin", "https://twitcasting.tv")
	defaultHeaders.Set("Referer", "https://twitcasting.tv/")
	cookie := os.Getenv("TC_COOKIE")
	defaultHeaders.Set("Cookie", cookie)
	deviceID := client.CookieValue(cookie, "web_devid")
	sessionID := client.CookieValue(cookie, "cs_session_id")
	log.Debug("TwitCasting cookie configuration", "set", cookie != "")
	return Client{
		httpClient: client.NewClient(),
		headers:    defaultHeaders,
		deviceID:   deviceID,
		sessionID:  sessionID,
	}
}

func (tc Client) Subscribe(ctx context.Context, userID string) error {
	endpoint := apiBaseURL + "/push/subscribe"
	payload := subscriptionPayload{
		SessionID: tc.sessionID,
		DeviceID:  tc.deviceID,
		User:      userID,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON payload: %w", err)
	}

	resp, err := tc.put(ctx, endpoint, tc.headers, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return fmt.Errorf("subscribe request: %w", err)
	}

	body, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	if readErr != nil {
		return fmt.Errorf("read subscribe response: %w", readErr)
	}

	if resp.StatusCode != http.StatusOK || strings.TrimSpace(string(body)) != "{}" {
		return fmt.Errorf(
			"subscribe failed: status=%d content_type=%q body=%q",
			resp.StatusCode,
			resp.Header.Get("Content-Type"),
			body,
		)
	}

	log.Info("subscription created", "user_id", userID)
	return nil
}

func (tc Client) Unsubscribe(ctx context.Context, userID string) error {
	endpoint := apiBaseURL + "/push/subscribe"
	payload := subscriptionPayload{
		SessionID: tc.sessionID,
		DeviceID:  tc.deviceID,
		User:      userID,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON payload: %w", err)
	}

	resp, err := tc.delete(ctx, endpoint, tc.headers, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return fmt.Errorf("unsubscribe request: %w", err)
	}

	body, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	if readErr != nil {
		return fmt.Errorf("read unsubscribe response: %w", readErr)
	}

	if resp.StatusCode != http.StatusOK || strings.TrimSpace(string(body)) != "{}" {
		return fmt.Errorf(
			"unsubscribe failed: status=%d content_type=%q body=%q",
			resp.StatusCode,
			resp.Header.Get("Content-Type"),
			body,
		)
	}

	log.Info("subscription removed", "user_id", userID)
	return nil
}

// ListSubscriptions lists the channels in the current TwitCasting session.
func (tc Client) ListSubscriptions(ctx context.Context) error {
	log.Debug("loading notifyindex.php")
	pageData, err := tc.loadPage(ctx)
	if err != nil {
		return err
	}

	users := parseUsers(pageData)
	fmt.Printf("Subscribed channels: %d\n", len(users))
	for _, user := range users {
		fmt.Println(user)
	}

	return nil
}

func (tc Client) put(ctx context.Context, url string, headers http.Header, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "PUT", url, body)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	if headers != nil {
		req.Header = headers.Clone()
	}
	// set default headers
	for k, vals := range tc.httpClient.DefaultHeaders {
		if req.Header.Get(k) == "" {
			for _, v := range vals {
				req.Header.Set(k, v)
			}
		}
	}

	resp, err := tc.httpClient.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending PUT request: %w", err)
	}

	return resp, nil
}

func (tc Client) delete(ctx context.Context, url string, headers http.Header, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, body)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	if headers != nil {
		req.Header = headers.Clone()
	}
	// set default headers
	for k, vals := range tc.httpClient.DefaultHeaders {
		if req.Header.Get(k) == "" {
			for _, v := range vals {
				req.Header.Set(k, v)
			}
		}
	}

	resp, err := tc.httpClient.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending DELETE request: %w", err)
	}

	return resp, nil
}
