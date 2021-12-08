package main

import (
	"fmt"
	"math/big"
	"math/rand"

	crand "crypto/rand"
)

const (
	digits         = "0123456789"
	uppers         = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	lowers         = "abcdefghijklmnopqrstuvwxyz"
	specials       = "~=+%^*[]{}!@#$|" // must exclude ?<>()/\& space ' "
	all            = digits + uppers + lowers + specials
	passwordLength = 12
)

type cryptoSource struct{}

var ErrCryptoSourceFailure = fmt.Errorf("error generating crpyto random source")

func (s cryptoSource) Seed(seed int64) {}

// Returns signed int64
func (s cryptoSource) Int63() int64 {
	max := ^uint(1 << 63)
	bigInt, err := crand.Int(crand.Reader, big.NewInt(int64(max)))
	if err != nil {
		panic(ErrCryptoSourceFailure)
	}
	return bigInt.Int64()
}

func getRandomPassword() string {
	var src cryptoSource
	rnd := rand.New(src)

	buf := make([]byte, passwordLength)
	buf[0] = digits[rnd.Intn(len(digits))]
	buf[1] = specials[rnd.Intn(len(specials))]
	buf[2] = uppers[rnd.Intn(len(uppers))]
	buf[3] = lowers[rnd.Intn(len(lowers))]
	for i := 4; i < passwordLength; i++ {
		buf[i] = all[rnd.Intn(len(all))]
	}

	// use crand for shuffle
	rand.Shuffle(len(buf), func(i, j int) {
		buf[i], buf[j] = buf[j], buf[i]
	})

	return string(buf)
}
