package mindp

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
)

type stdlibTransport struct {
	client  *http.Client
	persona Persona
	cfg     Config
}

func newStdlibTransport(cfg Config, persona Persona) TransportProvider {
	jar, _ := cookiejar.New(nil)
	transport := &http.Transport{}
	proxy := cfg.Stealth.Transport.Proxy
	if proxy == "" {
		proxy = cfg.Proxy
	}
	if proxy != "" {
		if proxyURL, err := url.Parse(proxy); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}
	return &stdlibTransport{
		client: &http.Client{
			Transport: transport,
			Jar:       jar,
		},
		persona: persona,
		cfg:     cfg,
	}
}

func (t *stdlibTransport) Do(ctx context.Context, req *TransportRequest) (*TransportResponse, error) {
	method := req.Method
	if method == "" {
		method = http.MethodGet
	}
	body := io.Reader(nil)
	if len(req.Body) > 0 {
		body = bytes.NewReader(req.Body)
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, req.URL, body)
	if err != nil {
		return nil, err
	}
	for k, vv := range req.Headers {
		for _, v := range vv {
			httpReq.Header.Add(k, v)
		}
	}
	for k, v := range t.cfg.Stealth.Transport.DefaultHeaders {
		if httpReq.Header.Get(k) == "" {
			httpReq.Header.Set(k, v)
		}
	}
	if t.cfg.Stealth.Transport.BindPersona {
		if httpReq.Header.Get("User-Agent") == "" {
			httpReq.Header.Set("User-Agent", effectiveUserAgent(t.cfg, t.persona))
		}
		if httpReq.Header.Get("Accept-Language") == "" {
			httpReq.Header.Set("Accept-Language", effectiveAcceptLanguage(t.cfg, t.persona))
		}
	}
	resp, err := t.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &TransportResponse{
		Status:  resp.StatusCode,
		Headers: resp.Header.Clone(),
		Body:    data,
	}, nil
}
