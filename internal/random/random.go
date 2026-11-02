// Package random supplies unbiased cryptographic randomness without mutable state.
package random

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
)

// Int returns a uniform integer in [0,n), or an entropy-source error.
func Int(n int) (int, error) {
	if n <= 0 {
		return 0, errors.New("random: nonpositive bound")
	}
	bound := uint64(n)
	threshold := -bound % bound
	var b [8]byte
	for {
		if _, err := rand.Read(b[:]); err != nil {
			return 0, err
		}
		x := binary.LittleEndian.Uint64(b[:])
		if x >= threshold {
			return int(x % bound), nil
		}
	}
}

// Float returns a uniform 53-bit fraction in [0,1).
func Float() (float64, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return float64(binary.LittleEndian.Uint64(b[:])>>11) / (1 << 53), nil
}
