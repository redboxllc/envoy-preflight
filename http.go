package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
)

func getServerInfo(ctx context.Context, url string) (ServerInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ServerInfo{}, err
	}
	req.Header.Set("Accept", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return ServerInfo{}, err
	}

	b, err := readBody(res)
	if err != nil {
		return ServerInfo{}, err
	}

	si := ServerInfo{}
	err = json.Unmarshal(b, &si)
	return si, err
}

func postKill(ctx context.Context, url string) (statusCode int, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return 0, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	_, _ = readBody(res)
	return res.StatusCode, nil
}

func readBody(r *http.Response) ([]byte, error) {
	defer r.Body.Close()
	return io.ReadAll(r.Body)
}
