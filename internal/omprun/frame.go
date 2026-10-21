package omprun

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

// maxReassembledBytes is omp's advertised maxReassembledFrameBytes (64 MiB).
const maxReassembledBytes = 64 << 20

// chunkFrame is one protocol-v2 segment of an oversized logical record.
type chunkFrame struct {
	ChunkID    string `json:"chunkId"`
	Index      int    `json:"index"`
	Count      int    `json:"count"`
	ByteLength int    `json:"byteLength"`
	Data       string `json:"data"`
}

// chunkAssembly is the in-progress chunk sequence. Only the stdout reader
// touches it.
type chunkAssembly struct {
	id         string
	next       int
	count      int
	byteLength int
	buf        []byte
}

// handleFrame accepts one physical stdout frame. Ordinary frames are logical
// records; rpc_chunk frames are validated and reassembled first. A sequence
// must be uninterrupted: any other frame, a gap, or a size mismatch drops the
// partial record and reports a protocol error rather than parsing a corrupt
// object.
func (s *Session) handleFrame(line []byte) {
	var probe struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(line, &probe) != nil {
		return // ignore unparseable noise
	}
	if probe.Type != "rpc_chunk" {
		if s.chunk != nil {
			s.chunk = nil
			s.emit(Event{Type: "diagnostic", Text: "omp protocol error: chunked record interrupted by another frame"})
		}
		s.dispatch(line, probe.Type)
		return
	}
	var c chunkFrame
	if err := json.Unmarshal(line, &c); err != nil {
		s.chunkFailure("malformed rpc_chunk frame")
		return
	}
	record, err := s.addChunk(c)
	if err != nil {
		s.chunkFailure(err.Error())
		return
	}
	if record != nil {
		s.handleRecord(record)
	}
}

// addChunk appends one chunk and returns the complete record once the last
// chunk arrives.
func (s *Session) addChunk(c chunkFrame) ([]byte, error) {
	a := s.chunk
	if a == nil {
		if c.Index != 0 {
			return nil, fmt.Errorf("rpc_chunk %s started at index %d", c.ChunkID, c.Index)
		}
		if c.ChunkID == "" || c.Count < 1 || c.ByteLength < 1 || c.ByteLength > maxReassembledBytes {
			return nil, fmt.Errorf("rpc_chunk %s has invalid count/byteLength", c.ChunkID)
		}
		a = &chunkAssembly{id: c.ChunkID, count: c.Count, byteLength: c.ByteLength, buf: make([]byte, 0, c.ByteLength)}
		s.chunk = a
	} else if c.ChunkID != a.id || c.Index != a.next || c.Count != a.count || c.ByteLength != a.byteLength {
		return nil, fmt.Errorf("rpc_chunk sequence %s interleaved or out of order", a.id)
	}
	segment, err := base64.StdEncoding.DecodeString(c.Data)
	if err != nil {
		return nil, fmt.Errorf("rpc_chunk %s has invalid base64", a.id)
	}
	if len(a.buf)+len(segment) > a.byteLength {
		return nil, fmt.Errorf("rpc_chunk %s exceeds its declared length", a.id)
	}
	a.buf = append(a.buf, segment...)
	a.next++
	if a.next < a.count {
		return nil, nil
	}
	s.chunk = nil
	if len(a.buf) != a.byteLength || !utf8.Valid(a.buf) || !json.Valid(a.buf) {
		return nil, fmt.Errorf("rpc_chunk %s did not reassemble into one JSON object", a.id)
	}
	return a.buf, nil
}

func (s *Session) chunkFailure(reason string) {
	s.chunk = nil
	s.emit(Event{Type: "diagnostic", Text: "omp protocol error: " + reason})
}
