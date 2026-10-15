package subscriptions

import (
	"context"
	"fmt"
	"io"
	"regexp"
)

func (tc Client) loadPage(ctx context.Context) ([]byte, error) {
	url := "https://twitcasting.tv/notifyindex.php"
	// notifyindex.php is account-specific. Send the same session cookie used by
	// the subscribe API, otherwise TwitCasting serves the empty/no-user page.
	resp, err := tc.httpClient.Get(ctx, url, tc.headers)
	if err != nil {
		return nil, fmt.Errorf("failed to get notifyindex.php from twitcasting.tv: %w", err)
	}

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get notifyindex.php with status=%d, body=%s", resp.StatusCode, string(body))
	}

	return body, nil
}

func parseUsers(pageData []byte) []string {
	re := regexp.MustCompile(
		`(?s)<span\s+class="tw-user-name-screen-name"\s*>(.*?)</span>`,
	)
	matches := re.FindAllSubmatch(pageData, -1)

	var subscribedUsers []string
	for _, match := range matches {
		subscribedUsers = append(subscribedUsers, string(match[1]))
	}

	return subscribedUsers
}
