package messageingress

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type TurnClient interface {
	SubmitTurn(context.Context, TurnSubmitRequest) (TurnSubmitResponse, error)
}

type HTTPKernelClient struct {
	BaseURL      string
	RuntimeToken string
	HTTPClient   *http.Client
}

func (c HTTPKernelClient) SubmitTurn(ctx context.Context, req TurnSubmitRequest) (TurnSubmitResponse, error) {
	if strings.TrimSpace(c.BaseURL) == "" {
		return TurnSubmitResponse{}, fmt.Errorf("kernel base url is required")
	}
	body, err := json.Marshal(req)
	if err != nil {
		return TurnSubmitResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/turn", bytes.NewReader(body))
	if err != nil {
		return TurnSubmitResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.RuntimeToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.RuntimeToken)
	}
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 120 * time.Second}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return TurnSubmitResponse{}, err
	}
	defer resp.Body.Close()
	content, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return TurnSubmitResponse{}, err
	}
	var turnResp TurnSubmitResponse
	if len(content) > 0 {
		_ = json.Unmarshal(content, &turnResp)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if turnResp.Error != nil {
			return turnResp, fmt.Errorf("kernel turn rejected: %s: %s", turnResp.Error.Code, turnResp.Error.Message)
		}
		return turnResp, fmt.Errorf("kernel turn request failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(content)))
	}
	if len(content) > 0 {
		if err := json.Unmarshal(content, &turnResp); err != nil {
			return TurnSubmitResponse{}, err
		}
	}
	return turnResp, nil
}
