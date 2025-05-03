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
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httputil"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// StatusRegex describes the pattern for a raw HTTP Response code.
var StatusRegex = regexp.MustCompile("(?i)(?:Status:|HTTP\\/[\\d\\.]+)\\s+(\\d{3}.*)")

// client implements a SCGI client, which is a standard for
// interfacing external applications with Web servers.
type client struct {
	rwc net.Conn
	// keepAlive bool // TODO: implement
	stderr bool
	logger *zap.Logger
}

// Do made the request and returns a io.Reader that translates the data read
// from scgi responder out of scgi packet before returning it.
func (c *client) Do(p map[string]string, req io.Reader) (r io.Reader, err error) {
	writer := &streamWriter{c: c}
	writer.buf = bufPool.Get().(*bytes.Buffer)
	writer.buf.Reset()
	defer bufPool.Put(writer.buf)

	err = writer.writeNetstring(p)
	if err != nil {
		return
	}

	if req != nil {
		_, err = io.Copy(writer, req)
		if err != nil {
			return nil, err
		}
	}

	r = &streamReader{c: c}
	return
}

// clientCloser is a io.ReadCloser. It wraps a io.Reader with a Closer
// that closes the client connection.
type clientCloser struct {
	rwc net.Conn
	r   *streamReader
	io.Reader

	status int
	logger *zap.Logger
}

func (s clientCloser) Close() error {
	stderr := s.r.stderr.Bytes()
	if len(stderr) == 0 {
		return s.rwc.Close()
	}

	logLevel := zapcore.WarnLevel
	if s.status >= 400 {
		logLevel = zapcore.ErrorLevel
	}

	if c := s.logger.Check(logLevel, "stderr"); c != nil {
		c.Write(zap.ByteString("body", stderr))
	}

	return s.rwc.Close()
}

// Request returns a HTTP Response with Header and Body
// from scgi responder
func (c *client) Request(p map[string]string, req io.Reader) (resp *http.Response, err error) {
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
		statusNumber, statusInfo, statusIsCut := strings.Cut(resp.Header.Get("Status"), " ")
		resp.StatusCode, err = strconv.Atoi(statusNumber)
		if err != nil {
			return
		}
		if statusIsCut {
			resp.Status = statusInfo
		}

	} else {
		// Pull the response status.
		var lineOne string
		lineOne, err = tp.ReadContinuedLine()
		if err != nil && err != io.EOF {
			return
		}
		statusLine := StatusRegex.FindStringSubmatch(lineOne)

		if len(statusLine) > 1 {
			statusNumber, statusInfo, statusIsCut := strings.Cut(statusLine[1], " ")
			resp.StatusCode, err = strconv.Atoi(statusNumber)
			if err != nil {
				return
			}
			if statusIsCut {
				resp.Status = statusInfo
			}

		} else {
			resp.StatusCode = http.StatusOK
		}
	}

	// TODO: fixTransferEncoding ?
	resp.TransferEncoding = resp.Header["Transfer-Encoding"]
	resp.ContentLength, _ = strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)

	// wrap the response body in our closer
	closer := clientCloser{
		rwc:    c.rwc,
		r:      r.(*streamReader),
		Reader: rb,
		status: resp.StatusCode,
		logger: noopLogger,
	}
	if chunked(resp.TransferEncoding) {
		closer.Reader = httputil.NewChunkedReader(rb)
	}
	if c.stderr {
		closer.logger = c.logger
	}
	resp.Body = closer

	return
}

// Get issues a GET request to the scgi responder.
func (c *client) Get(p map[string]string, body io.Reader, l int64) (resp *http.Response, err error) {
	p["REQUEST_METHOD"] = "GET"
	p["CONTENT_LENGTH"] = strconv.FormatInt(l, 10)

	return c.Request(p, body)
}

// Head issues a HEAD request to the scgi responder.
func (c *client) Head(p map[string]string) (resp *http.Response, err error) {
	p["REQUEST_METHOD"] = "HEAD"
	p["CONTENT_LENGTH"] = "0"

	return c.Request(p, nil)
}

// Options issues an OPTIONS request to the scgi responder.
func (c *client) Options(p map[string]string) (resp *http.Response, err error) {
	p["REQUEST_METHOD"] = "OPTIONS"
	p["CONTENT_LENGTH"] = "0"

	return c.Request(p, nil)
}

// Post issues a POST request to the scgi responder. with request body
// in the format that bodyType specified
func (c *client) Post(p map[string]string, method string, bodyType string, body io.Reader, l int64) (resp *http.Response, err error) {
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

// PostForm issues a POST to the scgi responder, with form
// as a string key to a list values (url.Values)
func (c *client) PostForm(p map[string]string, data url.Values) (resp *http.Response, err error) {
	body := bytes.NewReader([]byte(data.Encode()))
	return c.Post(p, "POST", "application/x-www-form-urlencoded", body, int64(body.Len()))
}

// PostFile issues a POST to the scgi responder in multipart(RFC 2046) standard,
// with form as a string key to a list values (url.Values),
// and/or with file as a string key to a list file path.
func (c *client) PostFile(p map[string]string, data url.Values, file map[string]string) (resp *http.Response, err error) {
	buf := &bytes.Buffer{}
	writer := multipart.NewWriter(buf)
	bodyType := writer.FormDataContentType()

	for key, val := range data {
		for _, v0 := range val {
			err = writer.WriteField(key, v0)
			if err != nil {
				return
			}
		}
	}

	for key, val := range file {
		fd, e := os.Open(val)
		if e != nil {
			return nil, e
		}
		defer fd.Close()

		part, e := writer.CreateFormFile(key, filepath.Base(val))
		if e != nil {
			return nil, e
		}
		_, err = io.Copy(part, fd)
		if err != nil {
			return
		}
	}

	err = writer.Close()
	if err != nil {
		return
	}

	return c.Post(p, "POST", bodyType, buf, int64(buf.Len()))
}

// SetReadTimeout sets the read timeout for future calls that read from the
// scgi responder. A zero value for t means no timeout will be set.
func (c *client) SetReadTimeout(t time.Duration) error {
	if t != 0 {
		return c.rwc.SetReadDeadline(time.Now().Add(t))
	}
	return nil
}

// SetWriteTimeout sets the write timeout for future calls that send data to
// the scgi responder. A zero value for t means no timeout will be set.
func (c *client) SetWriteTimeout(t time.Duration) error {
	if t != 0 {
		return c.rwc.SetWriteDeadline(time.Now().Add(t))
	}
	return nil
}

// Checks whether chunked is part of the encodings stack
func chunked(te []string) bool { return len(te) > 0 && te[0] == "chunked" }