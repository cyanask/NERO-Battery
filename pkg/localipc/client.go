// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package localipc

import (
	"context"
	"net"
	"net/http"
	"time"
)

// Client uses only the protected Unix endpoint. No proxy, DNS or TCP fallback.
func Client() *http.Client {
	return &http.Client{Timeout: 12 * time.Second, Transport: &http.Transport{
		DisableKeepAlives: true,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			p, err := LoadPolicy(DefaultPolicyPath)
			if err != nil {
				return nil, err
			}
			c, err := (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, "unix", DefaultSocketPath)
			if err != nil {
				return nil, err
			}
			u := c.(*net.UnixConn)
			if err := p.VerifyServer(u); err != nil {
				c.Close()
				return nil, err
			}
			return c, nil
		},
	}, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
}
