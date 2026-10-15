package fcm

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
)

// Firebase Installation ID：17 bytes，首 4 bit 固定 0111，base64url 取前 22 字元
func newFID() string {
	b := make([]byte, 17)
	rand.Read(b)
	b[0] = 0x70 | (b[0] & 0x0f)
	return b64.EncodeToString(b)[:22]
}

// 取得 Installations auth token：第一次建立 installation，之後用 refresh token 換新的。
func (fc FCMClient) installAuthToken(ctx context.Context, c *fcmConfig, s *State) (string, error) {
	base := fmt.Sprintf("https://firebaseinstallations.googleapis.com/v1/projects/%s/installations", c.ProjectID)
	key := http.Header{"x-goog-api-key": {c.APIKey}}

	if s.FID == "" {
		var r struct {
			FID          string `json:"fid"`
			RefreshToken string `json:"refreshToken"`
			AuthToken    struct {
				Token string `json:"token"`
			} `json:"authToken"`
		}
		in := map[string]string{
			"fid": newFID(), "authVersion": "FIS_v2",
			"appId": c.AppID, "sdkVersion": sdkVersion,
		}
		if err := fc.doJSON(ctx, base, key, in, &r); err != nil {
			return "", err
		}
		s.FID, s.RefreshToken = r.FID, r.RefreshToken
		if err := s.save(); err != nil {
			return "", err
		}
		return r.AuthToken.Token, nil
	}

	var r struct {
		Token string `json:"token"`
	}
	hdr := http.Header{
		"x-goog-api-key": {c.APIKey},
		"Authorization":  {"FIS " + s.RefreshToken},
	}
	in := map[string]any{"installation": map[string]string{"sdkVersion": sdkVersion, "appId": c.AppID}}
	if err := fc.doJSON(ctx, base+"/"+s.FID+"/authTokens:generate", hdr, in, &r); err != nil {
		return "", err
	}
	return r.Token, nil
}
