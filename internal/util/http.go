// Package util — 网络工具（零依赖，对应原 lib/http.js）
package util

import (
	"io"
	"net/http"
	"time"
)

// Response 简化的 HTTP 响应
type Response struct {
	Status int
	Body   string
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

// Get 发 GET 请求，返回状态码与响应体（超时 30s，与 JS 版一致）
func Get(url string, headers map[string]string) (*Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept-Encoding", "identity")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &Response{Status: resp.StatusCode, Body: string(body)}, nil
}
