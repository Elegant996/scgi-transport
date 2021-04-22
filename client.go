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

// Copyright 2012 Junqing Tan <ivan@mysqlab.net> and The Go Authors
// Use of this source code is governed by a BSD-style
// Part of source code is based on Go fcgi package

package scgi

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

// SCGIClient implements a FastCGI client, which is a standard for
// interfacing external applications with Web servers.
type SCGIClient struct {
	conn      net.Conn
	keepAlive bool
}

// DialWithDialerContext connects to the scgi responder at the specified network address, using custom net.Dialer
// and a context.
// See func net.Dial for a description of the network and address parameters.
func DialWithDialerContext(ctx context.Context, network, address string, dialer net.Dialer) (scgi *SCGIClient, err error) {
	var conn net.Conn
	conn, err = dialer.DialContext(ctx, network, address)
	if err != nil {
		return
	}

	scgi = &SCGIClient{
		conn:      conn,
		keepAlive: false,
	}

	return
}

// DialContext is like Dial but passes ctx to dialer.Dial.
func DialContext(ctx context.Context, network, address string) (scgi *SCGIClient, err error) {
	// TODO: why not set timeout here?
	return DialWithDialerContext(ctx, network, address, net.Dialer{})
}

// Dial connects to the scgi responder at the specified network address, using default net.Dialer.
// See func net.Dial for a description of the network and address parameters.
func Dial(network, address string) (scgi *SCGIClient, err error) {
	return DialContext(context.Background(), network, address)
}

// Close closes scgi connection
func (c *SCGIClient) Close() {
	c.conn.Close()
}

// writeNetstring writes the netstring to the writer.
func (c *SCGIClient) writeNetstring(content []byte) (err error) {
	if _, err := c.conn.Write([]byte(strconv.Itoa(len(content)))); err != nil {
		return err
	}
	if _, err := c.conn.Write([]byte{':'}); err != nil {
		return err
	}
	if _, err := c.conn.Write(content); err != nil {
		return err
	}
	_, err = c.conn.Write([]byte{','})
	return err
}

// writePairs writes all headers to the buffer. SCGI requires CONTENT_LENGTH as the first header.
func (c *SCGIClient) writePairs(pairs map[string]string) error {
	b := &bytes.Buffer{}
	var k string = "CONTENT_LENGTH"
	if v, ok := pairs[k]; ok {
		b.Grow(len(k) + len(v) + 2)
		if _, err := b.WriteString(k); err != nil {
			return err
		}
		if err := b.WriteByte(0x00); err != nil {
			return err
		}
		if _, err := b.WriteString(v); err != nil {
			return err
		}
		if err := b.WriteByte(0x00); err != nil {
			return err
		}
		delete(pairs, k)
	}
	for k, v := range pairs {
		b.Grow(len(k) + len(v) + 2)
		if _, err := b.WriteString(k); err != nil {
			return err
		}
		if err := b.WriteByte(0x00); err != nil {
			return err
		}
		if _, err := b.WriteString(v); err != nil {
			return err
		}
		if err := b.WriteByte(0x00); err != nil {
			return err
		}
	}
	return c.writeNetstring(b.Bytes())
}

// Do made the request and returns a io.Reader that translates the data read
// from scgi responder out of scgi packet before returning it.
func (c *SCGIClient) Do(p map[string]string, req io.Reader) (r io.Reader, err error) {
	err = c.writePairs(p)
	if err != nil {
		return
	}

	if req != nil {
		_, _ = io.Copy(c.conn, req)
	}

	r = c.conn
	return
}

// clientCloser is a io.ReadCloser. It wraps a io.Reader with a Closer
// that closes SCGIClient connection.
type clientCloser struct {
	*SCGIClient
	io.Reader
}

func (f clientCloser) Close() error { return f.conn.Close() }

// Request returns a HTTP Response with Header and Body
// from scgi responder
func (c *SCGIClient) Request(p map[string]string, req io.Reader) (resp *http.Response, err error) {
	r, err := c.Do(p, req)
	if err != nil {
		return
	}	

	rb := bufio.NewReader(r)
	tp := textproto.NewReader(rb)
	resp = new(http.Response)

	// Parse the response headers.
	mimeHeader, err := tp.ReadMIMEHeader()
	if err != nil && err != io.EOF {
		return
	}
	resp.Header = http.Header(mimeHeader)

	if resp.Header.Get("Status") != "" {
		statusParts := strings.SplitN(resp.Header.Get("Status"), " ", 2)
		resp.StatusCode, err = strconv.Atoi(statusParts[0])
		if err != nil {
			return
		}
		if len(statusParts) > 1 {
			resp.Status = statusParts[1]
		}

	} else {
		resp.StatusCode = http.StatusOK
	}

	// TODO: fixTransferEncoding ?
	resp.TransferEncoding = resp.Header["Transfer-Encoding"]
	resp.ContentLength, _ = strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)

	if chunked(resp.TransferEncoding) {
		resp.Body = clientCloser{c, httputil.NewChunkedReader(rb)}
	} else {
		resp.Body = clientCloser{c, ioutil.NopCloser(rb)}
	}
	return
}

// Get issues a GET request to the scgi responder.
func (c *SCGIClient) Get(p map[string]string, body io.Reader, l int64) (resp *http.Response, err error) {

	p["REQUEST_METHOD"] = "GET"
	p["CONTENT_LENGTH"] = strconv.FormatInt(l, 10)

	return c.Request(p, body)
}

// Head issues a HEAD request to the scgi responder.
func (c *SCGIClient) Head(p map[string]string) (resp *http.Response, err error) {

	p["REQUEST_METHOD"] = "HEAD"
	p["CONTENT_LENGTH"] = "0"

	return c.Request(p, nil)
}

// Options issues an OPTIONS request to the scgi responder.
func (c *SCGIClient) Options(p map[string]string) (resp *http.Response, err error) {

	p["REQUEST_METHOD"] = "OPTIONS"
	p["CONTENT_LENGTH"] = "0"

	return c.Request(p, nil)
}

// Post issues a POST request to the scgi responder. with request body
// in the format that bodyType specified
func (c *SCGIClient) Post(p map[string]string, method string, bodyType string, body io.Reader, l int64) (resp *http.Response, err error) {
	if p == nil {
		p = make(map[string]string)
	}

	p["REQUEST_METHOD"] = strings.ToUpper(method)

	if len(p["REQUEST_METHOD"]) == 0 || p["REQUEST_METHOD"] == "GET" {
		p["REQUEST_METHOD"] = "POST"
	}

	p["CONTENT_LENGTH"] = strconv.FormatInt(l, 10)
	if len(bodyType) > 0 {
		p["CONTENT_TYPE"] = bodyType
	} else {
		p["CONTENT_TYPE"] = "application/x-www-form-urlencoded"
	}

	return c.Request(p, body)
}

// SetReadTimeout sets the read timeout for future calls that read from the
// fcgi responder. A zero value for t means no timeout will be set.
func (c *SCGIClient) SetReadTimeout(t time.Duration) error {
	if t != 0 {
		return c.conn.SetReadDeadline(time.Now().Add(t))
	}
	return nil
}

// SetWriteTimeout sets the write timeout for future calls that send data to
// the fcgi responder. A zero value for t means no timeout will be set.
func (c *SCGIClient) SetWriteTimeout(t time.Duration) error {
	if t != 0 {
		return c.conn.SetWriteDeadline(time.Now().Add(t))
	}
	return nil
}

// Checks whether chunked is part of the encodings stack
func chunked(te []string) bool { return len(te) > 0 && te[0] == "chunked" }
