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
var src cryptoSource
var rnd = rand.New(src)

func (s cryptoSource) Seed(seed int64) {}

func (s cryptoSource) Int63() int64 {
	bigInt, err := crand.Int(crand.Reader, new(big.Int).SetUint64(1<<63))
	if err != nil {
		panic(ErrCryptoSourceFailure)
	}
	return bigInt.Int64()
}

func getRandomPassword() (passwd string, err error) {
	defer func() (string, error) {
		if rerr := recover(); rerr != nil && fmt.Sprint(rerr) == ErrCryptoSourceFailure.Error() {
			return "", ErrCryptoSourceFailure
		} else if rerr != nil {
			panic(rerr)
		}
		return passwd, nil
	}()

	buf := make([]byte, passwordLength)
	buf[0] = digits[rnd.Intn(len(digits))]
	buf[1] = specials[rnd.Intn(len(specials))]
	buf[2] = uppers[rnd.Intn(len(uppers))]
	buf[3] = lowers[rnd.Intn(len(lowers))]
	for i := 4; i < passwordLength; i++ {
		buf[i] = all[rnd.Intn(len(all))]
	}

	rnd.Shuffle(len(buf), func(i, j int) {
		buf[i], buf[j] = buf[j], buf[i]
	})

	passwd = string(buf)
	return passwd, nil
}
