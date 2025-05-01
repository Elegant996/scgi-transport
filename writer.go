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
	"bytes"
	"strconv"
)

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
	if v, ok := pairs["CONTENT_LENGTH"]; ok {
		w.buf.WriteString("CONTENT_LENGTH")
		w.buf.WriteByte(0x00)
		w.buf.WriteString(v)
		w.buf.WriteByte(0x00)
		delete(pairs, "CONTENT_LENGTH")
	}
	// write remaining headers
	for k, v := range pairs {
		w.buf.WriteString(k)
		w.buf.WriteByte(0x00)
		w.buf.WriteString(v)
		w.buf.WriteByte(0x00)
	}

	// writes the buffer length per SCGI requirements
	w.c.rwc.WriteString(strconv.Itoa(w.buf.Len()) + ":")

	w.buf.WriteByte(',')

	_, err := w.buf.WriteTo(w.c.rwc)
	return err
}