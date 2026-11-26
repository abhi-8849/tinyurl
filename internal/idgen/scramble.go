package idgen

import "math/bits"

// Multiplier is the Scramble key. It is odd and not a multiple of 31, so it shares no factor
// with 62^7 = 2^7 * 31^7 and Scramble is a bijection on [0, Space).
const Multiplier uint64 = 2_718_281_828_459

// Scramble maps id to id*Multiplier mod Space, so consecutive ids give codes that do not look sequential.
// Different ids below Space always give different results. The 128-bit product avoids overflow.
func Scramble(id uint64) uint64 {
	hi, lo := bits.Mul64(id, Multiplier)
	// hi < Multiplier < Space whenever id < 2^64, which Div64 requires.
	_, rem := bits.Div64(hi, lo, Space)
	return rem
}
