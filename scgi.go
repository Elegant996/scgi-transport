// Copyright 2015 Matthew Holt and The Caddy Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package scgi

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	// "github.com/caddyserver/caddy/v2/modules/caddytls"
	"go.uber.org/zap"

	"github.com/caddyserver/caddy/v2"
)

func init() {
	caddy.RegisterModule(Transport{})
}

// Transport facilitates SCGI communication.
type Transport struct {
	// The duration used to set a deadline when connecting to an upstream.
	DialTimeout caddy.Duration `json:"dial_timeout,omitempty"`

	// The duration used to set a deadline when reading from the SCGI server.
	ReadTimeout caddy.Duration `json:"read_timeout,omitempty"`

	// The duration used to set a deadline when sending to the SCGI server.
	WriteTimeout caddy.Duration `json:"write_timeout,omitempty"`

	serverSoftware string
	logger         *zap.Logger
}

// CaddyModule returns the Caddy module information.
func (Transport) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.reverse_proxy.transport.scgi",
		New: func() caddy.Module { return new(Transport) },
	}
}

// Provision sets up t.
func (t *Transport) Provision(ctx caddy.Context) error {
	t.logger = ctx.Logger(t)
	t.serverSoftware = "Caddy"
	if mod := caddy.GoModule(); mod.Version != "" {
		t.serverSoftware += "/" + mod.Version
	}
	return nil
}

// RoundTrip implements http.RoundTripper.
func (t Transport) RoundTrip(r *http.Request) (*http.Response, error) {
	env, err := t.buildEnv(r)
	if err != nil {
		return nil, fmt.Errorf("building environment: %v", err)
	}

	// TODO: doesn't dialer have a Timeout field?
	ctx := r.Context()
	if t.DialTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(t.DialTimeout))
		defer cancel()
	}

	// extract dial information from request (should have been embedded by the reverse proxy)
	network, address := "tcp", r.URL.Host
	if dialInfo, ok := reverseproxy.GetDialInfo(ctx); ok {
		network = dialInfo.Network
		address = dialInfo.Address
	}

	t.logger.Debug("roundtrip",
		zap.Object("request", caddyhttp.LoggableHTTPRequest{Request: r}),
		zap.String("dial", address),
		zap.Any("env", env), // TODO: this uses reflection I think
	)

	scgiBackend, err := DialContext(ctx, network, address)
	if err != nil {
		// TODO: wrap in a special error type if the dial failed, so retries can happen if enabled
		return nil, fmt.Errorf("dialing backend: %v", err)
	}
	// scgiBackend gets closed when response body is closed (see clientCloser)

	// read/write timeouts
	if err := scgiBackend.SetReadTimeout(time.Duration(t.ReadTimeout)); err != nil {
		return nil, fmt.Errorf("setting read timeout: %v", err)
	}
	if err := scgiBackend.SetWriteTimeout(time.Duration(t.WriteTimeout)); err != nil {
		return nil, fmt.Errorf("setting write timeout: %v", err)
	}

	contentLength := r.ContentLength
	if contentLength == 0 {
		contentLength, _ = strconv.ParseInt(r.Header.Get("Content-Length"), 10, 64)
	}

	var resp *http.Response
	switch r.Method {
	case http.MethodGet:
		resp, err = scgiBackend.Get(env, r.Body, contentLength)
	default:
		resp, err = scgiBackend.Post(env, r.Method, r.Body, contentLength)
	}

	return resp, err
}

// buildEnv returns a set of CGI environment variables for the request.
func (t Transport) buildEnv(r *http.Request) (map[string]string, error) {
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)

	var env map[string]string

	// Separate remote IP and port; more lenient than net.SplitHostPort
	var ip, port string
	if idx := strings.LastIndex(r.RemoteAddr, ":"); idx > -1 {
		ip = r.RemoteAddr[:idx]
		port = r.RemoteAddr[idx+1:]
	} else {
		ip = r.RemoteAddr
	}

	// Remove [] from IPv6 addresses
	ip = strings.Replace(ip, "[", "", 1)
	ip = strings.Replace(ip, "]", "", 1)

	// Get the request URL from context. The context stores the original URL in case
	// it was changed by a middleware such as rewrite. By default, we pass the
	// original URI in as the value of REQUEST_URI (the user can overwrite this
	// if desired). This is how nginx defaults: http://stackoverflow.com/a/12485156/1048862
	origReq, ok := r.Context().Value(caddyhttp.OriginalRequestCtxKey).(http.Request)
	if !ok {
		// some requests, like active health checks, don't add this to
		// the request context, so we can just use the current URL
		origReq = *r
	}
	reqURL := origReq.URL

	requestScheme := "http"

	reqHost, reqPort, err := net.SplitHostPort(r.Host)
	if err != nil {
		// whatever, just assume there was no port
		reqHost = r.Host
	}

	// Some variables are unused but cleared explicitly to prevent
	// the parent environment from interfering.
	env = map[string]string{
		// Variables defined in CGI 1.1 spec
		"AUTH_TYPE":         "", // Not used
		"CONTENT_LENGTH":    r.Header.Get("Content-Length"),
		"CONTENT_TYPE":      r.Header.Get("Content-Type"),
		"GATEWAY_INTERFACE": "CGI/1.1",
		"PATH_INFO":         "", // Not used
		"QUERY_STRING":      r.URL.RawQuery,
		"REMOTE_ADDR":       ip,
		"REMOTE_HOST":       ip, // For speed, remote host lookups disabled
		"REMOTE_PORT":       port,
		"REMOTE_IDENT":      "", // Not used
		"REMOTE_USER":       "", // TODO: once there are authentication handlers, populate this
		"REQUEST_METHOD":    r.Method,
		"REQUEST_SCHEME":    requestScheme,
		"SERVER_NAME":       reqHost,
		"SERVER_PORT":       reqPort,
		"SERVER_PROTOCOL":   r.Proto,
		"SERVER_SOFTWARE":   t.serverSoftware,

		// Other variables
		"HTTP_HOST":       r.Host, // added here, since not always part of headers
		"REQUEST_URI":     reqURL.RequestURI(),
		"SCGI":            "1", // Required
	}

	// Add all HTTP headers to env variables
	for field, val := range r.Header {
		header := strings.ToUpper(field)
		header = headerNameReplacer.Replace(header)
		env["HTTP_"+header] = strings.Join(val, ", ")
	}
	return env, nil
}

var headerNameReplacer = strings.NewReplacer(" ", "_", "-", "_")

// Interface guards
var (
	_ caddy.Provisioner = (*Transport)(nil)
	_ http.RoundTripper = (*Transport)(nil)
)