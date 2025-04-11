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
	"encoding/json"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
)

func init() {
	httpcaddyfile.RegisterDirective("scgi", parseSCGI)
}

// UnmarshalCaddyfile deserializes Caddyfile tokens into h.
//
//	transport scgi {
//	    root <path>
//	    split <at>
//	    env <key> <value>
//	    resolve_root_symlink
//	    dial_timeout <duration>
//	    read_timeout <duration>
//	    write_timeout <duration>
//	}
//
func (t *Transport) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		for d.NextBlock(0) {
			switch d.Val() {
			case "root":
				if !d.NextArg() {
					return d.ArgErr()
				}
				t.Root = d.Val()

			case "split":
				t.SplitPath = d.RemainingArgs()
				if len(t.SplitPath) == 0 {
					return d.ArgErr()
				}

			case "env":
				args := d.RemainingArgs()
				if len(args) != 2 {
					return d.ArgErr()
				}
				if t.EnvVars == nil {
					t.EnvVars = make(map[string]string)
				}
				t.EnvVars[args[0]] = args[1]

			case "resolve_root_symlink":
				if d.NextArg() {
					return d.ArgErr()
				}
				t.ResolveRootSymlink = true

			case "dial_timeout":
				if !d.NextArg() {
					return d.ArgErr()
				}
				dur, err := caddy.ParseDuration(d.Val())
				if err != nil {
					return d.Errf("bad timeout value %s: %v", d.Val(), err)
				}
				t.DialTimeout = caddy.Duration(dur)

			case "read_timeout":
				if !d.NextArg() {
					return d.ArgErr()
				}
				dur, err := caddy.ParseDuration(d.Val())
				if err != nil {
					return d.Errf("bad timeout value %s: %v", d.Val(), err)
				}
				t.ReadTimeout = caddy.Duration(dur)

			case "write_timeout":
				if !d.NextArg() {
					return d.ArgErr()
				}
				dur, err := caddy.ParseDuration(d.Val())
				if err != nil {
					return d.Errf("bad timeout value %s: %v", d.Val(), err)
				}
				t.WriteTimeout = caddy.Duration(dur)

			case "capture_stderr":
				if d.NextArg() {
					return d.ArgErr()
				}
				t.CaptureStderr = true

			default:
				return d.Errf("unrecognized subdirective %s", d.Val())
			}
		}
	}
	return nil
}

// parseSCGI parses the scgi directive, which has the same syntax
// as the reverse_proxy directive (in fact, the reverse_proxy's directive
// Unmarshaler is invoked by this function). A line such as this:
//
//	scgi localhost:7777
//
// is equivalent to a route consisting of:
//
//	reverse_proxy / localhost:7777 {
//		transport scgi {
//		}
//	}
//
// If this "common" config is not compatible with a user's requirements,
// they can use a manual approach based on the example above to configure
// it precisely as they need.
//
// If a matcher is specified by the user, for example:
//
//	scgi /subpath localhost:7777
//
// then the resulting handlers are wrapped in a subroute that uses the
// user's matcher as a prerequisite to enter the subroute. In other
// words, the directive's matcher is necessary, but not sufficient.
func parseSCGI(h httpcaddyfile.Helper) ([]httpcaddyfile.ConfigValue, error) {
	if !h.Next() {
		return nil, h.ArgErr()
	}

	// set up the transport for SCGI
	scgiTransport := Transport{}

	// if the user specified a matcher token, use that
	// matcher in a route that wraps both of our routes;
	// either way, strip the matcher token and pass
	// the remaining tokens to the unmarshaler so that
	// we can gain the rest of the reverse_proxy syntax
	userMatcherSet, err := h.ExtractMatcherSet()
	if err != nil {
		return nil, err
	}

	// make a new dispenser from the remaining tokens so that we
	// can reset the dispenser back to this point for the
	// reverse_proxy unmarshaler to read from it as well
	dispenser := h.NewFromNextSegment()

	// read the subdirectives that we allow as overrides to
	// the scgi shortcut
	// NOTE: we delete the tokens as we go so that the reverse_proxy
	// unmarshal doesn't see these subdirectives which it cannot handle
	for dispenser.Next() {
		for dispenser.NextBlock(0) {
			// ignore any sub-subdirectives that might
			// have the same name somewhere within
			// the reverse_proxy passthrough tokens
			if dispenser.Nesting() != 1 {
				continue
			}

			// parse the scgi subdirectives
			switch dispenser.Val() {
			case "root":
				if !dispenser.NextArg() {
					return nil, dispenser.ArgErr()
				}
				scgiTransport.Root = dispenser.Val()
				dispenser.DeleteN(2)

			case "split":
				extensions = dispenser.RemainingArgs()
				dispenser.DeleteN(len(extensions) + 1)
				if len(extensions) == 0 {
					return nil, dispenser.ArgErr()
				}

			case "env":
				args := dispenser.RemainingArgs()
				dispenser.DeleteN(len(args) + 1)
				if len(args) != 2 {
					return nil, dispenser.ArgErr()
				}
				if scgiTransport.EnvVars == nil {
					scgiTransport.EnvVars = make(map[string]string)
				}
				scgiTransport.EnvVars[args[0]] = args[1]

			case "resolve_root_symlink":
				args := dispenser.RemainingArgs()
				dispenser.DeleteN(len(args) + 1)
				scgiTransport.ResolveRootSymlink = true

			case "dial_timeout":
				if !dispenser.NextArg() {
					return nil, dispenser.ArgErr()
				}
				dur, err := caddy.ParseDuration(dispenser.Val())
				if err != nil {
					return nil, dispenser.Errf("bad timeout value %s: %v", dispenser.Val(), err)
				}
				scgiTransport.DialTimeout = caddy.Duration(dur)
				dispenser.DeleteN(2)

			case "read_timeout":
				if !dispenser.NextArg() {
					return nil, dispenser.ArgErr()
				}
				dur, err := caddy.ParseDuration(dispenser.Val())
				if err != nil {
					return nil, dispenser.Errf("bad timeout value %s: %v", dispenser.Val(), err)
				}
				scgiTransport.ReadTimeout = caddy.Duration(dur)
				dispenser.DeleteN(2)

			case "write_timeout":
				if !dispenser.NextArg() {
					return nil, dispenser.ArgErr()
				}
				dur, err := caddy.ParseDuration(dispenser.Val())
				if err != nil {
					return nil, dispenser.Errf("bad timeout value %s: %v", dispenser.Val(), err)
				}
				scgiTransport.WriteTimeout = caddy.Duration(dur)
				dispenser.DeleteN(2)

			case "capture_stderr":
				args := dispenser.RemainingArgs()
				dispenser.DeleteN(len(args) + 1)
				scgiTransport.CaptureStderr = true
			}
		}
	}

	// reset the dispenser after we're done so that the reverse_proxy
	// unmarshaler can read it from the start
	dispenser.Reset()

	// set up a route list that we'll append to
	routes := caddyhttp.RouteList{}

	// set the list of allowed path segments on which to split
	scgiTransport.SplitPath = extensions

	// create the reverse proxy handler which uses our SCGI transport
	rpHandler := &reverseproxy.Handler{
		TransportRaw: caddyconfig.JSONModuleObject(scgiTransport, "protocol", "scgi", nil),
	}

	// the rest of the config is specified by the user
	// using the reverse_proxy directive syntax
	err = rpHandler.UnmarshalCaddyfile(dispenser)
	if err != nil {
		return nil, err
	}
	err = rpHandler.FinalizeUnmarshalCaddyfile(h)
	if err != nil {
		return nil, err
	}

	// create the final reverse proxy route
	rpRoute := caddyhttp.Route{
		HandlersRaw:    []json.RawMessage{caddyconfig.JSONModuleObject(rpHandler, "handler", "reverse_proxy", nil)},
	}

	subroute := caddyhttp.Subroute{
		Routes: append(routes, rpRoute),
	}

	// the user's matcher is a prerequisite for ours, so
	// wrap ours in a subroute and return that
	if userMatcherSet != nil {
		return []httpcaddyfile.ConfigValue{
			{
				Class: "route",
				Value: caddyhttp.Route{
					MatcherSetsRaw: []caddy.ModuleMap{userMatcherSet},
					HandlersRaw:    []json.RawMessage{caddyconfig.JSONModuleObject(subroute, "handler", "subroute", nil)},
				},
			},
		}, nil
	}

	// otherwise, return the literal subroute instead of
	// individual routes, to ensure they stay together and
	// are treated as a single unit, without necessarily
	// creating an actual subroute in the output
	return []httpcaddyfile.ConfigValue{
		{
			Class: "route",
			Value: subroute,
		},
	}, nil
}