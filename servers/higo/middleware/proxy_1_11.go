// +build go1.11

package middleware

import (
	"fmt"
	"net/http"
	"net/http/httputil"

	"github.com/sereiner/higo"
)

func proxyHTTP(tgt *ProxyTarget, c higo.Context, config ProxyConfig) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(tgt.URL)
	proxy.ErrorHandler = func(resp http.ResponseWriter, req *http.Request, err error) {
		desc := tgt.URL.String()
		if tgt.Name != "" {
			desc = fmt.Sprintf("%s(%s)", tgt.Name, tgt.URL.String())
		}
		c.Logger().Errorf("remote %s unreachable, could not forward: %v", desc, err)
		c.Error(higo.NewHTTPError(http.StatusServiceUnavailable))
	}
	proxy.Transport = config.Transport
	return proxy
}
