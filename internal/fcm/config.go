package fcm

import "os"

const sdkVersion = "w:0.6.4" // 與網站 Installations 請求 body 內的值一致即可

type fcmConfig struct {
	APIKey, ProjectID, AppID, VAPIDKey string
}

// 必要：FCM_API_KEY、FCM_PROJECT_ID、FCM_APP_ID。
// 選用：FCM_VAPID_KEY（網站 getToken 有指定 vapidKey 才需要；
// 你抓到的 registrations body 沒有 applicationPubKey，代表用的是預設金鑰，留空即可）。
func fcmConfigFromEnv() *fcmConfig {
	c := &fcmConfig{
		APIKey:    os.Getenv("FCM_API_KEY"),
		ProjectID: os.Getenv("FCM_PROJECT_ID"),
		AppID:     os.Getenv("FCM_APP_ID"),
		VAPIDKey:  os.Getenv("FCM_VAPID_KEY"),
	}
	if c.APIKey == "" || c.ProjectID == "" || c.AppID == "" {
		return nil
	}
	return c
}
