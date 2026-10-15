package fcm

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

// decryptAESGCM handles the legacy Web Push aesgcm content encoding, where
// encryption metadata is carried in the websocket headers rather than body.
func decryptAESGCM(data []byte, headers map[string]string, uaPriv *ecdh.PrivateKey, authSecret []byte) ([]byte, error) {
	var dhEncoded, saltEncoded string
	for _, item := range strings.Split(headers["crypto_key"], ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(item), "=")
		if ok && key == "dh" {
			dhEncoded = value
		}
	}
	for _, item := range strings.Split(headers["encryption"], ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(item), "=")
		if ok && key == "salt" {
			saltEncoded = value
		}
	}
	dh, err := b64.DecodeString(dhEncoded)
	if err != nil {
		return nil, fmt.Errorf("sender key encoding: %w", err)
	}
	salt, err := b64.DecodeString(saltEncoded)
	if err != nil {
		return nil, fmt.Errorf("salt encoding: %w", err)
	}
	if len(salt) != 16 {
		return nil, fmt.Errorf("bad salt length: %d", len(salt))
	}
	asPub, err := ecdh.P256().NewPublicKey(dh)
	if err != nil {
		return nil, fmt.Errorf("sender key: %w", err)
	}
	secret, err := uaPriv.ECDH(asPub)
	if err != nil {
		return nil, err
	}
	uaPub := uaPriv.PublicKey().Bytes()
	keyInfo := append([]byte("WebPush: info\x00"), uaPub...)
	keyInfo = append(keyInfo, dh...)
	prkKey, err := hkdf.Extract(sha256.New, secret, authSecret)
	if err != nil {
		return nil, err
	}
	ikm, err := hkdf.Expand(sha256.New, prkKey, string(keyInfo), 32)
	if err != nil {
		return nil, err
	}
	prk, err := hkdf.Extract(sha256.New, ikm, salt)
	if err != nil {
		return nil, err
	}
	cek, err := hkdf.Expand(sha256.New, prk, "Content-Encoding: aesgcm\x00", 16)
	if err != nil {
		return nil, err
	}
	nonce, err := hkdf.Expand(sha256.New, prk, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(data) < 18 {
		return nil, errors.New("encrypted payload too short")
	}
	plain, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return nil, err
	}
	// aesgcm plaintext starts with a two-byte padding length, then padding.
	if len(plain) < 2 {
		return nil, errors.New("plaintext too short")
	}
	padLen := int(binary.BigEndian.Uint16(plain[:2]))
	if padLen > len(plain)-2 {
		return nil, errors.New("bad padding length")
	}
	return plain[2+padLen:], nil
}

// ---------- RFC 8291 / 8188 (aes128gcm) ----------

func decrypt(body []byte, uaPriv *ecdh.PrivateKey, authSecret []byte) ([]byte, error) {
	// header: salt(16) | rs(4) | idlen(1) | keyid(idlen)
	if len(body) < 21 {
		return nil, errors.New("body too short")
	}
	salt := body[:16]
	rs := binary.BigEndian.Uint32(body[16:20])
	idlen := int(body[20])
	if len(body) < 21+idlen || rs < 18 {
		return nil, errors.New("bad header")
	}
	asPubRaw := body[21 : 21+idlen]
	data := body[21+idlen:]

	asPub, err := ecdh.P256().NewPublicKey(asPubRaw)
	if err != nil {
		return nil, fmt.Errorf("sender key: %w", err)
	}
	secret, err := uaPriv.ECDH(asPub)
	if err != nil {
		return nil, err
	}
	uaPub := uaPriv.PublicKey().Bytes()

	prkKey, err := hkdf.Extract(sha256.New, secret, authSecret)
	if err != nil {
		return nil, err
	}
	keyInfo := append([]byte("WebPush: info\x00"), uaPub...)
	keyInfo = append(keyInfo, asPubRaw...)
	ikm, err := hkdf.Expand(sha256.New, prkKey, string(keyInfo), 32)
	if err != nil {
		return nil, err
	}
	prk, err := hkdf.Extract(sha256.New, ikm, salt)
	if err != nil {
		return nil, err
	}
	cek, err := hkdf.Expand(sha256.New, prk, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		return nil, err
	}
	nonceBase, err := hkdf.Expand(sha256.New, prk, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		return nil, err
	}
	blk, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(blk)
	if err != nil {
		return nil, err
	}

	var out []byte
	for seq := uint64(0); len(data) > 0; seq++ {
		n := int(rs)
		last := false
		if len(data) <= n {
			n, last = len(data), true
		}
		nonce := bytes.Clone(nonceBase)
		var ctr [8]byte
		binary.BigEndian.PutUint64(ctr[:], seq)
		for i := 0; i < 8; i++ {
			nonce[4+i] ^= ctr[i]
		}
		plain, err := gcm.Open(nil, nonce, data[:n], nil)
		if err != nil {
			return nil, fmt.Errorf("record %d: %w", seq, err)
		}
		// strip padding: trailing zeros, then delimiter (0x01 or 0x02 for last)
		i := len(plain) - 1
		for i >= 0 && plain[i] == 0 {
			i--
		}
		if i < 0 {
			return nil, errors.New("missing padding delimiter")
		}
		if last && plain[i] != 2 || !last && plain[i] != 1 {
			return nil, errors.New("bad padding delimiter")
		}
		out = append(out, plain[:i]...)
		data = data[n:]
	}
	return out, nil
}
