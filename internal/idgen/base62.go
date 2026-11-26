package idgen

// CodeLen is the length of a generated short code.
const CodeLen = 7

// Space is 62^CodeLen, the number of distinct generated codes (about 3.5 trillion).
const Space uint64 = 3_521_614_606_208

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// Encode writes id in Base62, left-padded with '0' to width characters.
// An id that needs more than width characters is returned in full, never truncated.
func Encode(id uint64, width int) string {
	var buf [20]byte // 62^11 > 2^64, so 11 suffices; 20 leaves room for any width we pad to.
	i := len(buf)
	for id > 0 {
		i--
		buf[i] = alphabet[id%62]
		id /= 62
	}
	for len(buf)-i < width && i > 0 {
		i--
		buf[i] = alphabet[0]
	}
	return string(buf[i:])
}
