package fcm

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"

	"github.com/google/uuid"
)

var b64 = base64.RawURLEncoding

// ---------- state ----------

type State struct {
	UAID      string `json:"uaid"`
	ChannelID string `json:"channel_id"`
	Endpoint  string `json:"endpoint"`
	Priv      string `json:"priv"` // base64url raw P-256 private key
	Auth      string `json:"auth"` // base64url 16 bytes

	FID          string `json:"fid,omitempty"`           // Firebase Installation ID
	RefreshToken string `json:"refresh_token,omitempty"` // Installations refresh token
	FCMToken     string `json:"fcm_token,omitempty"`
	SiteToken    string `json:"site_token,omitempty"` // 已回報給網站的 FCM token
}

func statePath() string {
	if p := os.Getenv("STATE_FILE"); p != "" {
		return p
	}
	return "state.json" // 容器內由 Containerfile 的 STATE_FILE 覆蓋為 /data/state.json
}

func LoadState() (*State, error) {
	b, err := os.ReadFile(statePath())
	if err == nil {
		var s State
		if err := json.Unmarshal(b, &s); err != nil {
			return nil, err
		}
		return &s, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	k, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	auth := make([]byte, 16)
	rand.Read(auth)
	s := &State{
		ChannelID: uuid.NewString(),
		Priv:      b64.EncodeToString(k.Bytes()),
		Auth:      b64.EncodeToString(auth),
	}
	return s, s.save()
}

func (s *State) save() error {
	b, _ := json.MarshalIndent(s, "", "  ")
	tmp := statePath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, statePath())
}

func (s *State) key() (*ecdh.PrivateKey, error) {
	raw, err := b64.DecodeString(s.Priv)
	if err != nil {
		return nil, err
	}
	return ecdh.P256().NewPrivateKey(raw)
}
