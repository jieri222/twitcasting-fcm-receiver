package fcm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
	"twitcasting-fcm-receiver/internal/log"

	"github.com/coder/websocket"
)

type msg struct {
	MessageType  string            `json:"messageType"`
	UAID         string            `json:"uaid,omitempty"`
	ChannelID    string            `json:"channelID,omitempty"`
	ChannelIDs   []string          `json:"channelIDs,omitempty"`
	Key          string            `json:"key,omitempty"` // VAPID 公鑰 (base64url)，register 時使用
	UseWebPush   bool              `json:"use_webpush,omitempty"`
	Status       int               `json:"status,omitempty"`
	PushEndpoint string            `json:"pushEndpoint,omitempty"`
	Version      string            `json:"version,omitempty"`
	Data         string            `json:"data,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	Updates      []update          `json:"updates,omitempty"`
}

type update struct {
	ChannelID string `json:"channelID"`
	Version   string `json:"version"`
	Code      int    `json:"code"`
}

var webhookHTTPClient = &http.Client{Timeout: 15 * time.Second}

func sendWebSocketMessage(ctx context.Context, c *websocket.Conn, m msg) error {
	b, _ := json.Marshal(m)
	log.Debug("push websocket send", "message_type", m.MessageType)
	return c.Write(ctx, websocket.MessageText, b)
}

func handleNotification(plain []byte) {
	fmt.Printf("%s\n", plain) // 一行一則，方便 podman logs / 管線處理
	if u := os.Getenv("WEBHOOK_URL"); u != "" {
		go func() {
			req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(plain))
			if err != nil {
				log.Warn("could not create webhook request", "error", err)
				return
			}
			req.Header.Set("Content-Type", "application/octet-stream")
			resp, err := webhookHTTPClient.Do(req)
			if err != nil {
				log.Warn("webhook delivery failed", "error", err)
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
				log.Warn("webhook returned unsuccessful status", "status", resp.StatusCode)
				return
			}
			log.Debug("webhook delivered", "payload_bytes", len(plain))
		}()
	}
}

func RunSession(ctx context.Context, s *State, url string, ready chan<- struct{}) error {
	started := time.Now()
	phase := "loading push state"
	lastMessageType := ""
	log.Debug("push websocket session starting")

	priv, err := s.key()
	if err != nil {
		return err
	}
	auth, _ := b64.DecodeString(s.Auth)

	phase = "connecting"
	c, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{
		HTTPClient: http.DefaultClient,
		HTTPHeader: http.Header{"Origin": {"https://updates.push.services.mozilla.com"}},
		OnPingReceived: func(_ context.Context, payload []byte) bool {
			log.Debug("websocket control ping received; auto pong enabled",
				"payload", fmt.Sprintf("%q", payload),
				"payload_len", len(payload),
			)
			return true // Keep coder/websocket's automatic Pong response enabled.
		},
		OnPongReceived: func(_ context.Context, payload []byte) {
			log.Debug("websocket control pong received",
				"payload", fmt.Sprintf("%q", payload),
				"payload_len", len(payload),
			)
		},
	})
	if err != nil {
		return err
	}
	phase = "connected"
	log.Debug("push websocket connected")
	defer c.CloseNow()
	c.SetReadLimit(1 << 20)

	hello := msg{MessageType: "hello", UseWebPush: true, UAID: s.UAID, ChannelIDs: []string{}}
	if s.Endpoint != "" {
		hello.ChannelIDs = []string{s.ChannelID}
	}
	if err := sendWebSocketMessage(ctx, c, hello); err != nil {
		return err
	}
	phase = "waiting for server message"

	// Application-level keepalive used by the legacy Mozilla Push protocol.
	pongCh := make(chan struct{}, 1)
	markPong := func() {
		select {
		case pongCh <- struct{}{}:
		default:
		}
	}
	go func() {
		t := time.NewTicker(4 * time.Minute)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				log.Debug("push websocket send keepalive", "payload", "{}")
				if err := c.Write(ctx, websocket.MessageText, []byte("{}")); err != nil {
					log.Warn("push websocket keepalive failed", "error", err)
					_ = c.CloseNow()
					return
				}
				sentAt := time.Now()
				timeout := time.NewTimer(10 * time.Second)
				select {
				case <-ctx.Done():
					timeout.Stop()
					return
				case <-pongCh:
					timeout.Stop()
					log.Debug("push application ping acknowledged", "round_trip", time.Since(sentAt).Round(time.Millisecond))
				case <-timeout.C:
					log.Warn("push application ping timed out", "timeout", 10*time.Second)
					_ = c.CloseNow()
					return
				}
			}
		}
	}()

	fc := NewFCMClient()
	for {
		// An idle push session can go longer than 15 minutes without an
		// application message. The periodic WebSocket Ping checks liveness;
		// use the session context here so an idle connection is not mistaken
		// for a failed one.
		_, raw, err := c.Read(ctx)
		if err != nil {
			status := websocket.CloseStatus(err)
			log.Error("push websocket read failed",
				"error", err,
				"close_status", status,
				"phase", phase,
				"last_message_type", lastMessageType,
				"session_for", time.Since(started).Round(time.Second),
			)
			return err
		}
		log.Debug("push websocket frame received", "bytes", len(raw))
		if string(raw) == "{}" {
			markPong()
			continue
		}
		var m msg
		if err := json.Unmarshal(raw, &m); err != nil {
			log.Error("bad push websocket json", "error", err)
			continue
		}
		lastMessageType = m.MessageType
		phase = "handling " + m.MessageType

		switch m.MessageType {
		case "hello":
			log.Debug("push session hello received")
			log.Debug("push hello state",
				"uaid_changed", m.UAID != s.UAID,
				"endpoint_exists", s.Endpoint != "",
				"fcm_token_exists", s.FCMToken != "",
			)
			if m.UAID != s.UAID {
				// 新帳號或伺服器端已重置：舊 endpoint 失效
				s.UAID, s.Endpoint, s.FCMToken = m.UAID, "", ""
				if err := s.save(); err != nil {
					return fmt.Errorf("save state after push hello: %w", err)
				}
			}
			if s.Endpoint == "" {
				if err := sendWebSocketMessage(ctx, c, msg{MessageType: "register", ChannelID: s.ChannelID, Key: os.Getenv("VAPID_KEY")}); err != nil {
					return err
				}
			}
			log.Debug("ensuring FCM registration", "endpoint_exists", s.Endpoint != "", "fcm_token_exists", s.FCMToken != "")
			if err := fc.ensureFCM(ctx, s, priv); err != nil {
				return fmt.Errorf("ensure FCM after hello: %w", err)
			}
			log.Debug("FCM registration check completed")

			if ready != nil {
				select {
				case ready <- struct{}{}:
				default:
				}
			}
			phase = "session ready"
		case "register":
			if m.Status != 200 {
				return fmt.Errorf("register failed: status %d", m.Status)
			}
			s.Endpoint = m.PushEndpoint
			if err := s.save(); err != nil {
				return fmt.Errorf("save state after push register: %w", err)
			}
			log.Debug("push endpoint registered")
			log.Debug("push registration saved", "p256dh_available", len(priv.PublicKey().Bytes()) > 0, "auth_available", s.Auth != "")
			log.Debug("ensuring FCM registration", "endpoint_exists", s.Endpoint != "", "fcm_token_exists", s.FCMToken != "")
			if err := fc.ensureFCM(ctx, s, priv); err != nil {
				return fmt.Errorf("ensure FCM after register: %w", err)
			}
			log.Debug("FCM registration check completed")

			if ready != nil {
				select {
				case ready <- struct{}{}:
				default:
				}
			}
			phase = "session ready"
		case "notification":
			log.Debug("push notification received")
			if m.Data != "" {
				body, err := b64.DecodeString(m.Data)
				if err == nil {
					var plain []byte
					if m.Headers["encoding"] == "aesgcm" {
						plain, err = decryptAESGCM(body, m.Headers, priv, auth)
					} else {
						plain, err = decrypt(body, priv, auth)
					}
					if err == nil {
						handleNotification(plain)
					}
				}
				if err != nil {
					log.Error("decrypt failed", "error", err) // 仍然 ack，避免無限重送
				}
			}
			ack := msg{MessageType: "ack", Updates: []update{{m.ChannelID, m.Version, 100}}}
			if err := sendWebSocketMessage(ctx, c, ack); err != nil {
				return err
			}
			phase = "session ready"
		case "ping":
			// Autopush can reply to the application's {} ping with this
			// application-level pong. Firefox treats it as a ping response.
			markPong()
			phase = "session ready"
		}
	}
}
