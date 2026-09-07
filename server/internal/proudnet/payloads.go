package proudnet

func uint64LE(value uint64) []byte {
	return appendUint64(nil, value)
}

func int32LE(value int32) []byte {
	return appendUint32(nil, uint32(value))
}

func appendUint16(dst []byte, value uint16) []byte {
	return append(dst, byte(value), byte(value>>8))
}

func appendUint32(dst []byte, value uint32) []byte {
	return append(dst, byte(value), byte(value>>8), byte(value>>16), byte(value>>24))
}

func appendUint64(dst []byte, value uint64) []byte {
	return append(dst,
		byte(value), byte(value>>8), byte(value>>16), byte(value>>24),
		byte(value>>32), byte(value>>40), byte(value>>48), byte(value>>56),
	)
}
