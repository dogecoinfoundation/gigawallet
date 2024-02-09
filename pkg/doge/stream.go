package doge

type Stream struct {
	b []byte
	p uint64
}

func (s *Stream) bytes(num uint64) []byte {
	p := s.p
	s.p += num
	return s.b[p : p+num]
}

func (s *Stream) uint16le() uint16 {
	b := s.b
	p := s.p
	s.p += 2
	_ = b[p+1] // bounds check hint to compiler; see golang.org/issue/14808
	return uint16(b[p]) | uint16(b[p+1])<<8
}

func (s *Stream) uint32le() uint32 {
	b := s.b
	p := s.p
	s.p += 4
	_ = b[p+3] // bounds check hint
	return uint32(b[p]) | uint32(b[p+1])<<8 | uint32(b[p+2])<<16 | uint32(b[p+3])<<24
}

func (s *Stream) uint64le() uint64 {
	b := s.b
	p := s.p
	s.p += 8
	_ = b[p+7] // bounds check hint
	return uint64(b[p]) | uint64(b[p+1])<<8 | uint64(b[p+2])<<16 | uint64(b[p+3])<<24 |
		uint64(b[p+4])<<32 | uint64(b[p+5])<<40 | uint64(b[p+6])<<48 | uint64(b[p+7])<<56
}

func (s *Stream) var_uint() uint64 {
	val := s.b[s.p]
	s.p += 1
	if val < 253 {
		return uint64(val)
	}
	if val == 253 {
		return uint64(s.uint16le())
	}
	if val == 254 {
		return uint64(s.uint32le())
	}
	return s.uint64le()
}
