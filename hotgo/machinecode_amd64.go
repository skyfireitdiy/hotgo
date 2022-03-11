package hotgo

func makeMachineCode(addr uint64) []byte {
	code := []byte{0x48, 0xba, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0x22}

	code[2] = byte(addr)
	code[3] = byte(addr >> 8)
	code[4] = byte(addr >> 16)
	code[5] = byte(addr >> 24)
	code[6] = byte(addr >> 32)
	code[7] = byte(addr >> 40)
	code[8] = byte(addr >> 48)
	code[9] = byte(addr >> 56)

	return code
}
