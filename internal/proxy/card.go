package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
)

func rewriteCard(resp *http.Response, publicURL string) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()

	var card map[string]any
	if err := json.Unmarshal(body, &card); err != nil {
		// Not JSON we understand: pass it through untouched.
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return nil
	}

	if _, ok := card["url"]; ok {
		card["url"] = publicURL
	}
	for _, key := range []string{"supportedInterfaces", "additionalInterfaces"} {
		items, _ := card[key].([]any)
		for _, item := range items {
			if m, ok := item.(map[string]any); ok {
				if _, has := m["url"]; has {
					m["url"] = publicURL
				}
			}
		}
	}

	out, err := json.Marshal(card)
	if err != nil {
		return err
	}
	resp.Body = io.NopCloser(bytes.NewReader(out))
	resp.ContentLength = int64(len(out))
	resp.Header.Set("Content-Length", strconv.Itoa(len(out)))
	return nil
}
