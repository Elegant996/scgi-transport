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
	"maps"
	"strconv"

	"github.com/jub0bs/iterutil"
)

// streamWriter abstracts out the separation of a stream into discrete netstrings.
type streamWriter struct {
	c       *client
	buf     *bytes.Buffer
	count	int64
}

func (w *streamWriter) Write(p []byte) (n int, err error) {
	n, err = w.buf.Write(p)
	w.count += int64(n)
	return
}

// writeNetstring writes all headers to the buffer
func (w *streamWriter) writeNetstring(pairs map[string]string) error {
	w.count = 0
	if v, ok := pairs["CONTENT_LENGTH"]; ok {
		w.buf.WriteString("CONTENT_LENGTH")
		w.buf.WriteByte(0x00)
		w.count++
		w.buf.WriteString(v)
		w.buf.WriteByte(0x00)
		w.count++
	}
	headers := maps.All(pairs)
	clFilter := func(h string, _ string) bool { return h != "CONTENT_LENGTH" }
	for k, v := range iterutil.Filter2(headers, clFilter) {
		w.buf.WriteString(k)
		w.buf.WriteByte(0x00)
		w.count++
		w.buf.WriteString(v)
		w.buf.WriteByte(0x00)
		w.count++
	}

	// store string before resetting buffer
	s := w.buf.String()
	w.buf.Reset()

	// write the netstring
	w.buf.WriteString(strconv.FormatInt(w.count, 10))
	w.buf.WriteByte(':')
	w.buf.WriteString(s)
	w.buf.WriteByte(',')

	_, err := w.buf.WriteTo(w.c.rwc)
	return err
}