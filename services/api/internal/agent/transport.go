package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var ErrInvalidTransport = errors.New("invalid agent transport configuration")

// HTTPSource is the narrow mTLS transport used by the worker. The HTTP client
// must be configured with ClientTLSConfig; this type never disables TLS checks.
type HTTPSource struct {
	baseURL   *url.URL
	client    *http.Client
	gatewayID string
}

func NewHTTPSource(rawURL, gatewayID string, client *http.Client) (*HTTPSource, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" || gatewayID == "" || strings.ContainsAny(gatewayID, "/?#") {
		return nil, ErrInvalidTransport
	}
	if client == nil {
		client = &http.Client{
			Timeout:       15 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
	}
	return &HTTPSource{baseURL: parsed, client: client, gatewayID: gatewayID}, nil
}

func (s *HTTPSource) endpoint(suffix string) string {
	copyURL := *s.baseURL
	copyURL.Path = strings.TrimRight(copyURL.Path, "/") + "/v1/agent/gateways/" + url.PathEscape(s.gatewayID) + suffix
	return copyURL.String()
}

func (s *HTTPSource) Claim(ctx context.Context) (Delivery, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.endpoint("/snapshots/next"), nil)
	if err != nil {
		return Delivery{}, err
	}
	response, err := s.client.Do(request)
	if err != nil {
		return Delivery{}, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNoContent {
		return Delivery{}, ErrNoDelivery
	}
	if response.StatusCode != http.StatusOK {
		return Delivery{}, statusError(response)
	}
	var delivery Delivery
	if err := decodeLimited(response.Body, &delivery); err != nil {
		return Delivery{}, err
	}
	if delivery.ID == "" || delivery.Generation <= 0 || len(delivery.Payload) == 0 || len(delivery.Signature) != 64 {
		return Delivery{}, ErrInvalidTransport
	}
	return delivery, nil
}

func (s *HTTPSource) Ack(ctx context.Context, deliveryID string, generation int64) error {
	return s.post(ctx, "/snapshots/"+url.PathEscape(deliveryID)+"/ack", map[string]int64{"generation": generation})
}

func (s *HTTPSource) Retry(ctx context.Context, deliveryID string, retryAt time.Time, reason string) error {
	if retryAt.IsZero() || deliveryID == "" {
		return ErrInvalidTransport
	}
	return s.post(ctx, "/snapshots/"+url.PathEscape(deliveryID)+"/retry", map[string]string{"retryAt": retryAt.UTC().Format(time.RFC3339Nano), "reason": reason})
}

func (s *HTTPSource) post(ctx context.Context, suffix string, body any) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint(suffix), bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return statusError(response)
	}
	return nil
}

func decodeLimited(reader io.Reader, value any) error {
	decoder := json.NewDecoder(io.LimitReader(reader, 4<<20))
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("decode transport response: %w", err)
	}
	return nil
}

func statusError(response *http.Response) error {
	return fmt.Errorf("agent transport status %s", response.Status+" ("+strconv.Itoa(response.StatusCode)+")")
}
