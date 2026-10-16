package proxy

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
)

func rewriteCard(resp *http.Response, from, to string) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	body = bytes.ReplaceAll(body, []byte(from), []byte(to))
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
	return nil
}
