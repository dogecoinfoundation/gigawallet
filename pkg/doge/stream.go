package doge

// Stream is a stream of bytes with read methods.
type Stream struct {
	buf []byte
	pos uint64
	len uint64
}

// NewStream creates a new stream from a byte slice.
func NewStream(buf []byte) *Stream {
	return &Stream{buf: buf, len: uint64(len(buf))}
}

// Valid returns true if all reads have succeeded without reading past the end of the data.
func (s *Stream) Valid() bool {
	return s.pos <= s.len
}

// Complete returns true if all the data has been read; implies Valid()
func (s *Stream) Complete() bool {
	return s.pos == s.len
}

// Bytes reads num bytes from the stream and returns them.
func (s *Stream) Bytes(num uint64) []byte {
	pos := s.pos
	s.pos += num
	if pos+num <= s.len {
		return s.buf[pos : pos+num]
	}
	return nil
}

// Uint16le reads a 16-bit unsigned integer from the stream in little-endian order.
func (s *Stream) Uint16le() uint16 {
	buf := s.buf
	pos := s.pos
	s.pos += 2
	if pos+2 <= s.len {
		_ = buf[pos+1] // bounds check hint to compiler; see golang.org/issue/14808
		return uint16(buf[pos]) | uint16(buf[pos+1])<<8
	}
	return 0
}

// Uint32le reads a 32-bit unsigned integer from the stream in little-endian order.
func (s *Stream) Uint32le() uint32 {
	buf := s.buf
	pos := s.pos
	s.pos += 4
	if pos+4 <= s.len {
		_ = buf[pos+3] // bounds check hint
		return uint32(buf[pos]) | uint32(buf[pos+1])<<8 | uint32(buf[pos+2])<<16 | uint32(buf[pos+3])<<24
	}
	return 0
}

// Uint64le reads a 64-bit unsigned integer from the stream in little-endian order.
func (s *Stream) Uint64le() uint64 {
	buf := s.buf
	pos := s.pos
	s.pos += 8
	if pos+8 <= s.len {
		_ = buf[pos+7] // bounds check hint
		return uint64(buf[pos]) | uint64(buf[pos+1])<<8 | uint64(buf[pos+2])<<16 | uint64(buf[pos+3])<<24 |
			uint64(buf[pos+4])<<32 | uint64(buf[pos+5])<<40 | uint64(buf[pos+6])<<48 | uint64(buf[pos+7])<<56
	}
	return 0
}

// VarUint reads a variable-length unsigned integer from the stream.
func (s *Stream) VarUint() uint64 {
	pos := s.pos
	s.pos += 1
	if pos+1 <= s.len {
		val := s.buf[pos]
		if val < 253 {
			return uint64(val)
		}
		if val == 253 {
			return uint64(s.Uint16le())
		}
		if val == 254 {
			return uint64(s.Uint32le())
		}
		return s.Uint64le()
	}
	return 0
}
