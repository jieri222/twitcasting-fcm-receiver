package fcm

import (
	"context"
	"fmt"
	"net/http"
)

func (fc FCMClient) registerFCM(ctx context.Context, c *fcmConfig, s *State, p256dh []byte) error {
	tok, err := fc.installAuthToken(ctx, c, s)
	if err != nil {
		return fmt.Errorf("installations: %w", err)
	}
	web := map[string]string{
		"endpoint": s.Endpoint,
		"auth":     s.Auth,
		"p256dh":   b64.EncodeToString(p256dh),
	}
	if c.VAPIDKey != "" {
		web["applicationPubKey"] = c.VAPIDKey
	}
	var r struct {
		Token string `json:"token"`
	}
	hdr := http.Header{
		"x-goog-api-key":                     {c.APIKey},
		"x-goog-firebase-installations-auth": {"FIS " + tok},
	}
	url := fmt.Sprintf("https://fcmregistrations.googleapis.com/v1/projects/%s/registrations", c.ProjectID)
	if err := fc.doJSON(ctx, url, hdr, map[string]any{"web": web}, &r); err != nil {
		return fmt.Errorf("registrations: %w", err)
	}
	s.FCMToken = r.Token
	return s.save()
}
