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
	"iter"
	"maps"
	"strconv"
	"strings"
)

// streamWriter abstracts out the separation of a stream into discrete netstrings.
type streamWriter struct {
	c   *client
	buf *bytes.Buffer
}

func (w *streamWriter) Write(p []byte) (int, error) {
	return w.buf.Write(p)
}

func (w *streamWriter) writeNetstring(pairs map[string]string) error {
	var sb strings.Builder
	nn := 0
	if v, ok := pairs["CONTENT_LENGTH"]; ok {
		n, _ := sb.WriteString("CONTENT_LENGTH")
		sb.WriteByte(0x00)
		m, _ := sb.WriteString(v)
		sb.WriteByte(0x00)
		nn += n + m + 2
	}

	headers := maps.All(pairs)
	clStr := func(h string, _ string) bool { return h != "CONTENT_LENGTH" }
	for k, v := range Filter2(headers, clStr) {
		n, _ := sb.WriteString(k)
		sb.WriteByte(0x00)
		m, _ := sb.WriteString(v)
		sb.WriteByte(0x00)
		nn += n + m + 2
	}

	// write the netstring
	w.buf.WriteString(strconv.Itoa(nn))
	w.buf.WriteByte(':')
	w.buf.WriteString(sb.String())
	w.buf.WriteByte(',')

	return w.FlushStream()
}

// FlushStream flush data then end current stream
func (w *streamWriter) FlushStream() error {
	_, err := w.buf.WriteTo(w.c.rwc)
	return err
}

// Filter returns an iterator composed of the pairs of seq that
// satisfy predicate p.
func Filter2[K, V any](seq iter.Seq2[K, V], p func(K, V) bool) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for k, v := range seq {
			if p(k, v) && !yield(k, v) {
				return
			}
		}
	}
}
