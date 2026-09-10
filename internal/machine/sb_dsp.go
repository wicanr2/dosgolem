package machine

// SoundBlasterDSP 是公開 DSP 埠契約的受限子集。透過回呼啟動受限 DMA；尚無音訊輸出。
type SoundBlasterDSP struct {
	SpeakerOn                      bool
	RateNumerator, RateDenominator uint64
	StartAutoDMA                   func(uint32) bool
	BlockSize                      uint32
	BlockSizeKnown                 bool
	pending                        uint8
	lengthLow                      byte
	lengthHighNext                 bool
	StartDMA                       func(uint32) bool
	CancelDMA                      func()
	IRQPending                     bool
	TimeConstant                   uint8
	TimeConstantKnown              bool
	reset                          bool
	mixerIndex                     uint8
	reply                          []byte
}

func (s *SoundBlasterDSP) In8(port uint16) (uint8, bool) {
	switch port {
	case 0x224:
		return s.mixerIndex, true
	case 0x225:
		switch s.mixerIndex {
		case 0x82:
			if s.IRQPending {
				return 1, true
			}
			return 0, true
		case 0x80:
			return 4, true
		case 0x81:
			return 0x22, true
		}
		return 0, false
	case 0x226:
		return 0xff, true
	case 0x22e:
		s.IRQPending = false
		if len(s.reply) > 0 {
			return 0x80, true
		}
		return 0, true
	case 0x22a:
		if len(s.reply) == 0 {
			return 0xff, true
		}
		v := s.reply[0]
		s.reply = s.reply[1:]
		return v, true
	case 0x22c:
		if s.reset {
			return 0x80, true
		}
		return 0, true
	}
	return 0, false
}
func (s *SoundBlasterDSP) Out8(port uint16, v uint8) bool {
	if port == 0x224 {
		s.mixerIndex = v
		return true
	}
	if port == 0x22c && !s.reset {
		if s.pending == 0x41 {
			if !s.lengthHighNext {
				s.lengthLow = v
				s.lengthHighNext = true
				return true
			}
			rate := uint64(s.lengthLow)<<8 | uint64(v)
			if rate < 5000 || rate > 45000 {
				return false
			}
			s.RateNumerator = rate
			s.RateDenominator = 1
			s.TimeConstantKnown = false
			s.pending = 0
			s.lengthHighNext = false
			return true
		}
		if s.pending == 0 && v == 0x41 {
			s.pending = v
			s.lengthHighNext = false
			return true
		}
		if s.pending == 0 {
			switch v {
			case 0xd1, 0xd3:
				s.SpeakerOn = v == 0xd1
				return true
			case 0xd8:
				value := byte(0)
				if s.SpeakerOn {
					value = 0xff
				}
				s.reply = append(s.reply, value)
				return true
			}
		}
		if s.pending == 0 && v == 0x1c {
			return s.BlockSizeKnown && s.StartAutoDMA != nil && s.StartAutoDMA(s.BlockSize)
		}
		if s.pending == 0x14 || s.pending == 0x48 {
			if !s.lengthHighNext {
				s.lengthLow = v
				s.lengthHighNext = true
				return true
			}
			n := uint32(s.lengthLow) | uint32(v)<<8
			if s.pending == 0x48 {
				s.BlockSize = n + 1
				s.BlockSizeKnown = true
			} else if s.StartDMA == nil || !s.StartDMA(n+1) {
				return false
			}
			s.pending = 0
			s.lengthHighNext = false
			return true
		}
		if (v == 0x14 || v == 0x48) && s.pending == 0 {
			s.pending = v
			s.lengthHighNext = false
			return true
		}
		if s.pending == 0x40 {
			s.TimeConstant = v
			s.TimeConstantKnown = true
			s.RateNumerator = 1000000
			s.RateDenominator = 256 - uint64(v)
			s.pending = 0
			return true
		}
		if v == 0x40 {
			s.pending = v
			return true
		}
	}
	if port == 0x22c && !s.reset && v == 0xe1 {
		s.reply = []byte{4, 5}
		return true
	}
	if port != 0x226 || v > 1 {
		return false
	}
	if v == 1 {
		s.reset = true
		s.SpeakerOn = false
		s.pending = 0
		s.lengthHighNext = false
		s.IRQPending = false
		if s.CancelDMA != nil {
			s.CancelDMA()
		}
		s.TimeConstantKnown = false
		s.RateNumerator = 22050
		s.RateDenominator = 1
		s.BlockSizeKnown = false
		s.reply = nil
		return true
	}
	if s.reset {
		s.reset = false
		s.reply = []byte{0xaa}
	}
	return true
}
