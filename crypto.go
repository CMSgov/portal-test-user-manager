package main

import (
	"encoding/binary"
	"log"
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

func (s cryptoSource) Seed(seed int64) {}

// Returns signed int64
func (s cryptoSource) Int63() int64 {
	return int64(s.Uint64() & ^uint64(1<<63))
}

func (s cryptoSource) Uint64() uint64 {
	var v uint64
	err := binary.Read(crand.Reader, binary.BigEndian, &v)
	if err != nil {
		log.Fatal(err) // TODO: handle this by cleaning up row in file and saving file
	}

	return v
}

func getRandomPassword(length int) string {
	if length < 8 {
		length = passwordLength
	}

	var src cryptoSource
	rnd := rand.New(src)

	buf := make([]byte, length)
	buf[0] = digits[rnd.Intn(len(digits))]
	buf[1] = specials[rnd.Intn(len(specials))]
	buf[2] = uppers[rnd.Intn(len(uppers))]
	buf[3] = lowers[rnd.Intn(len(lowers))]
	for i := 4; i < length; i++ {
		buf[i] = all[rnd.Intn(len(all))]
	}
	rand.Shuffle(len(buf), func(i, j int) {
		buf[i], buf[j] = buf[j], buf[i]
	})

	return string(buf)
}
