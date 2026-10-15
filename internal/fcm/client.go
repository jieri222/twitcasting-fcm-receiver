package fcm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"twitcasting-fcm-receiver/internal/client"
)

type FCMClient struct {
	Client *client.Client
}

func NewFCMClient() FCMClient {
	return FCMClient{
		Client: client.NewClient(),
	}
}

func (fc FCMClient) doJSON(ctx context.Context, url string, hdr http.Header, in, out any) error {
	var rd io.Reader
	if in != nil {
		b, _ := json.Marshal(in)
		rd = bytes.NewReader(b)
	}

	hdr.Set("Content-Type", "application/json")
	hdr.Set("Accept", "application/json")

	resp, err := fc.Client.Post(ctx, url, hdr, rd)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("%s %s: %d %s", "POST", url, resp.StatusCode, body)
	}
	if out != nil {
		return json.Unmarshal(body, out)
	}
	return nil
}
