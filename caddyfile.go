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
//     transport scgi {
//         env <key> <value>
//     }
//
func (t *Transport) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		for d.NextBlock(0) {
			switch d.Val() {
			case "env":
				args := d.RemainingArgs()
				if len(args) != 2 {
					return d.ArgErr()
				}
				if t.EnvVars == nil {
					t.EnvVars = make(map[string]string)
				}
				t.EnvVars[args[0]] = args[1]

			default:
				return d.Errf("unrecognized subdirective %s", d.Val())
			}
		}
	}
	return nil
}

// parseSCGI parses the scgi directive, which has the same syntax
// as the reverse_proxy directive (in fact, the reverse_proxy's directive
// Unmarshaler is invoked by this function) but the resulting proxy is specially
// configured to define SCRIPT_NAME to match the URI path. A line such as this:
//
//     scgi localhost:7777
//
// is equivalent to a route consisting of:
//
//     reverse_proxy / localhost:7777 {
//         transport scgi {
//             env SCRIPT_NAME {http.request.uri.path}
//         }
//     }
//
// If this "common" config is not compatible with a user's requirements,
// they can use a manual approach based on the example above to configure
// it precisely as they need.
//
// If a matcher is specified by the user, for example:
//
//     scgi /subpath localhost:7777
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
			switch dispenser.Val() {
			case "env":
				args := dispenser.RemainingArgs()
				dispenser.Delete()
				for range args {
					dispenser.Delete()
				}
				if len(args) != 2 {
					return nil, dispenser.ArgErr()
				}
				if scgiTransport.EnvVars == nil {
					scgiTransport.EnvVars = make(map[string]string)
				}
				scgiTransport.EnvVars[args[0]] = args[1]
		}
	}

	// reset the dispenser after we're done so that the reverse_proxy
	// unmarshaler can read it from the start
	dispenser.Reset()

	// set up a route list that we'll append to
	routes := caddyhttp.RouteList{}

	// create the reverse proxy handler which uses our SCGI transport
	rpHandler := &reverseproxy.Handler{
		TransportRaw: caddyconfig.JSONModuleObject(scgiTransport, "protocol", "scgi", nil),
	}

	// the rest of the config is specified by the user
	// using the reverse_proxy directive syntax
	// TODO: this can overwrite our scgiTransport that we encoded and
	// set on the rpHandler... even with a non-scgi transport!
	err = rpHandler.UnmarshalCaddyfile(h.Dispenser)
	if err != nil {
		return nil, err
	}

	// create the final reverse proxy route
	rpRoute := caddyhttp.Route{
		MatcherSetsRaw: []caddy.ModuleMap{rpMatcherSet},
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