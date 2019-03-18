package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/sereiner/higo"
)

type (
	// CORSConfig defines the config for CORS middleware.
	CORSConfig struct {
		// Skipper defines a function to skip middleware.
		Skipper Skipper

		// AllowOrigin defines a list of origins that may access the resource.
		// Optional. Default value []string{"*"}.
		AllowOrigins []string `yaml:"allow_origins"`

		// AllowMethods defines a list methods allowed when accessing the resource.
		// This is used in response to a preflight request.
		// Optional. Default value DefaultCORSConfig.AllowMethods.
		AllowMethods []string `yaml:"allow_methods"`

		// AllowHeaders defines a list of request headers that can be used when
		// making the actual request. This is in response to a preflight request.
		// Optional. Default value []string{}.
		AllowHeaders []string `yaml:"allow_headers"`

		// AllowCredentials indicates whether or not the response to the request
		// can be exposed when the credentials flag is true. When used as part of
		// a response to a preflight request, this indicates whether or not the
		// actual request can be made using credentials.
		// Optional. Default value false.
		AllowCredentials bool `yaml:"allow_credentials"`

		// ExposeHeaders defines a whitelist headers that clients are allowed to
		// access.
		// Optional. Default value []string{}.
		ExposeHeaders []string `yaml:"expose_headers"`

		// MaxAge indicates how long (in seconds) the results of a preflight request
		// can be cached.
		// Optional. Default value 0.
		MaxAge int `yaml:"max_age"`
	}
)

var (
	// DefaultCORSConfig is the default CORS middleware config.
	DefaultCORSConfig = CORSConfig{
		Skipper:      DefaultSkipper,
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodPatch, http.MethodPost, http.MethodDelete},
	}
)

// CORS returns a Cross-Origin Resource Sharing (CORS) middleware.
// See: https://developer.mozilla.org/en/docs/Web/HTTP/Access_control_CORS
func CORS() higo.MiddlewareFunc {
	return CORSWithConfig(DefaultCORSConfig)
}

// CORSWithConfig returns a CORS middleware with config.
// See: `CORS()`.
func CORSWithConfig(config CORSConfig) higo.MiddlewareFunc {
	// Defaults
	if config.Skipper == nil {
		config.Skipper = DefaultCORSConfig.Skipper
	}
	if len(config.AllowOrigins) == 0 {
		config.AllowOrigins = DefaultCORSConfig.AllowOrigins
	}
	if len(config.AllowMethods) == 0 {
		config.AllowMethods = DefaultCORSConfig.AllowMethods
	}

	allowMethods := strings.Join(config.AllowMethods, ",")
	allowHeaders := strings.Join(config.AllowHeaders, ",")
	exposeHeaders := strings.Join(config.ExposeHeaders, ",")
	maxAge := strconv.Itoa(config.MaxAge)

	return func(next higo.HandlerFunc) higo.HandlerFunc {
		return func(c higo.Context) error {
			if config.Skipper(c) {
				return next(c)
			}

			req := c.Request()
			res := c.Response()
			origin := req.Header.Get(higo.HeaderOrigin)
			allowOrigin := ""

			// Check allowed origins
			for _, o := range config.AllowOrigins {
				if o == "*" && config.AllowCredentials {
					allowOrigin = origin
					break
				}
				if o == "*" || o == origin {
					allowOrigin = o
					break
				}
			}

			// Simple request
			if req.Method != http.MethodOptions {
				res.Header().Add(higo.HeaderVary, higo.HeaderOrigin)
				res.Header().Set(higo.HeaderAccessControlAllowOrigin, allowOrigin)
				if config.AllowCredentials {
					res.Header().Set(higo.HeaderAccessControlAllowCredentials, "true")
				}
				if exposeHeaders != "" {
					res.Header().Set(higo.HeaderAccessControlExposeHeaders, exposeHeaders)
				}
				return next(c)
			}

			// Preflight request
			res.Header().Add(higo.HeaderVary, higo.HeaderOrigin)
			res.Header().Add(higo.HeaderVary, higo.HeaderAccessControlRequestMethod)
			res.Header().Add(higo.HeaderVary, higo.HeaderAccessControlRequestHeaders)
			res.Header().Set(higo.HeaderAccessControlAllowOrigin, allowOrigin)
			res.Header().Set(higo.HeaderAccessControlAllowMethods, allowMethods)
			if config.AllowCredentials {
				res.Header().Set(higo.HeaderAccessControlAllowCredentials, "true")
			}
			if allowHeaders != "" {
				res.Header().Set(higo.HeaderAccessControlAllowHeaders, allowHeaders)
			} else {
				h := req.Header.Get(higo.HeaderAccessControlRequestHeaders)
				if h != "" {
					res.Header().Set(higo.HeaderAccessControlAllowHeaders, h)
				}
			}
			if config.MaxAge > 0 {
				res.Header().Set(higo.HeaderAccessControlMaxAge, maxAge)
			}
			return c.NoContent(http.StatusNoContent)
		}
	}
}
