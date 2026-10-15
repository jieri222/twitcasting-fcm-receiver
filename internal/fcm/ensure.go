package fcm

import (
	"context"
	"crypto/ecdh"
	"fmt"
	"net/http"
	"os"
	"time"
	"twitcasting-fcm-receiver/internal/client"
	"twitcasting-fcm-receiver/internal/log"
)

// 已有 endpoint 時執行：缺 FCM token 就註冊，再把 token 回報給網站。
// 失敗只記 log，下次連線（重連時）會再試。
func (fc FCMClient) ensureFCM(ctx context.Context, s *State, priv *ecdh.PrivateKey) error {
	c := fcmConfigFromEnv()
	if c == nil {
		log.Warn("FCM registration skipped: configuration is incomplete", "endpoint_exists", s.Endpoint != "")
		return nil
	}
	if s.Endpoint == "" {
		log.Debug("FCM registration skipped: push endpoint is not available yet")
		return nil
	}
	if s.FCMToken == "" {
		rctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		log.Debug("registering FCM token")
		err := fc.registerFCM(rctx, c, s, priv.PublicKey().Bytes())
		cancel()
		if err != nil {
			return fmt.Errorf("register FCM token: %w", err)
		}
		log.Debug("FCM registration token acquired")
	}
	log.Debug("checking TwitCasting FCM registration", "site_token_matches", s.SiteToken == s.FCMToken, "cookie_set", os.Getenv("TC_COOKIE") != "")
	if err := fc.ensureSite(ctx, s); err != nil {
		return err
	}
	return nil
}

// 對應網站前端的 POST /push/register/fcm（需登入 Cookie）。
// TC_COOKIE：完整 Cookie header 字串（含登入 session 與 web_devid）。
// TC_DEVICE_ID：未設定時改用 Cookie 內的 web_devid（請確認兩者是否相同）。
func (fc FCMClient) ensureSite(ctx context.Context, s *State) error {
	cookie := os.Getenv("TC_COOKIE")
	if cookie == "" || s.FCMToken == "" || s.SiteToken == s.FCMToken {
		return nil
	}
	dev := os.Getenv("TC_DEVICE_ID")
	if dev == "" {
		dev = client.CookieValue(cookie, "web_devid")
	}
	if dev == "" {
		log.Warn("site registration skipped: device ID is missing")
		return fmt.Errorf("site registration: TC_DEVICE_ID is unset and web_devid is missing from TC_COOKIE")
	}
	rctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	hdr := http.Header{
		"Cookie":  {cookie},
		"Origin":  {"https://twitcasting.tv"},
		"Referer": {"https://twitcasting.tv/"},
	}
	in := map[string]string{"device_id": dev, "token": s.FCMToken}
	if err := fc.doJSON(rctx, "https://frontendapi.twitcasting.tv/push/register/fcm", hdr, in, nil); err != nil {
		return fmt.Errorf("site registration: %w", err) // 401/403 多半是登入 Cookie 過期
	}
	s.SiteToken = s.FCMToken
	if err := s.save(); err != nil {
		return fmt.Errorf("save state: %w", err)
	}
	log.Debug("site registration completed")
	return nil
}
