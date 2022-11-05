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
	"io"
	"bytes"
	"strconv"
)

// SCGI requires content length as the first header
const FirstHeaderKey string = "CONTENT_LENGTH"

// streamWriter abstracts out the separation of a stream into discrete netstrings.
type streamWriter struct {
	c       *client
	buf     *bytes.Buffer
}

func (w *streamWriter) Write(p []byte) (int, error) {
	return w.buf.Write(p)
}

// writeNetstring writes all headers to the buffer
func (w *streamWriter) writeNetstring(pairs map[string]string) error {
	if v, ok := pairs[FirstHeaderKey]; ok {
		w.buf.WriteString(FirstHeaderKey)
		w.buf.WriteByte(0x00)
		w.buf.WriteString(v)
		w.buf.WriteByte(0x00)
		delete(pairs, FirstHeaderKey)
	}
	// write remaining headers
	for k, v := range pairs {
		w.buf.WriteString(k)
		w.buf.WriteByte(0x00)
		w.buf.WriteString(v)
		w.buf.WriteByte(0x00)
	}

	if err := w.writeLength(); err != nil {
		return err
	}
	w.buf.WriteByte(',')

	return w.FlushStream()
}

// writeLength writes the buffer length to front of the writer
func (w *streamWriter) writeLength() error {
	s := strconv.Itoa(w.buf.Len()) + ":"
	_, err := io.WriteString(w.c.rwc, s)
	return err
}

// Flush write buffer data to the underlying connection
func (w *streamWriter) FlushStream() error {
	_, err := w.buf.WriteTo(w.c.rwc)
	return err
}