// Unix MD5-crypt / SHA-crypt routines used for Entware shadow verification.
// Algorithm structure is adapted from github.com/GehirnInc/crypt (BSD license).
// Copyright (c) 2012, Jeramey Crawford <jeramey@antihe.ro>.
// See THIRD_PARTY_NOTICES.md for the complete license notice.

package main

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"errors"
	"fmt"
	"hash"
	"strconv"
	"strings"
)

const cryptAlphabet = "./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func verifyUnixCrypt(stored, password string) (bool, error) {
	if stored == "" {
		return password == "", nil
	}
	if strings.HasPrefix(stored, "!") || strings.HasPrefix(stored, "*") {
		return false, errors.New("account password is locked")
	}

	var generated string
	var err error
	switch {
	case strings.HasPrefix(stored, "$1$"):
		generated, err = md5Crypt(password, stored)
	case strings.HasPrefix(stored, "$5$"):
		generated, err = shaCrypt(password, stored, "$5$", sha256.New, 32)
	case strings.HasPrefix(stored, "$6$"):
		generated, err = shaCrypt(password, stored, "$6$", sha512.New, 64)
	default:
		return false, fmt.Errorf("unsupported Entware password hash")
	}
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare([]byte(generated), []byte(stored)) == 1, nil
}

func parseCryptSalt(raw, prefix string, maxSalt int, allowRounds bool) (salt []byte, rounds int, explicitRounds bool, err error) {
	if !strings.HasPrefix(raw, prefix) {
		return nil, 0, false, errors.New("invalid crypt prefix")
	}
	rest := strings.TrimPrefix(raw, prefix)
	tokens := strings.Split(rest, "$")
	if len(tokens) < 1 {
		return nil, 0, false, errors.New("invalid crypt salt")
	}

	rounds = 5000
	saltToken := tokens[0]
	if allowRounds && strings.HasPrefix(saltToken, "rounds=") {
		if len(tokens) < 2 {
			return nil, 0, false, errors.New("invalid crypt rounds")
		}
		n, parseErr := strconv.Atoi(strings.TrimPrefix(saltToken, "rounds="))
		if parseErr != nil {
			return nil, 0, false, errors.New("invalid crypt rounds")
		}
		if n < 1000 {
			n = 1000
		}
		if n > 999999999 {
			n = 999999999
		}
		rounds = n
		explicitRounds = true
		saltToken = tokens[1]
	}
	if maxSalt > 0 && len(saltToken) > maxSalt {
		saltToken = saltToken[:maxSalt]
	}
	if saltToken == "" && prefix != "$1$" {
		return nil, 0, false, errors.New("empty crypt salt")
	}
	return []byte(saltToken), rounds, explicitRounds, nil
}

func repeatByteSequence(src []byte, size int) []byte {
	if size <= 0 {
		return nil
	}
	out := make([]byte, size)
	for i := 0; i < size; i++ {
		out[i] = src[i%len(src)]
	}
	return out
}

func cryptBase64(src []byte) []byte {
	if len(src) == 0 {
		return []byte{}
	}
	dstlen := (len(src)*8 + 5) / 6
	dst := make([]byte, dstlen)
	di, si := 0, 0
	n := len(src) / 3 * 3
	for si < n {
		val := uint(src[si+2])<<16 | uint(src[si+1])<<8 | uint(src[si])
		dst[di] = cryptAlphabet[val&0x3f]
		dst[di+1] = cryptAlphabet[val>>6&0x3f]
		dst[di+2] = cryptAlphabet[val>>12&0x3f]
		dst[di+3] = cryptAlphabet[val>>18]
		di += 4
		si += 3
	}
	rem := len(src) - si
	if rem == 0 {
		return dst
	}
	val := uint(src[si])
	if rem == 2 {
		val |= uint(src[si+1]) << 8
	}
	dst[di] = cryptAlphabet[val&0x3f]
	dst[di+1] = cryptAlphabet[val>>6&0x3f]
	if rem == 2 {
		dst[di+2] = cryptAlphabet[val>>12]
	}
	return dst
}

func shaCrypt(password, raw, prefix string, newHash func() hash.Hash, digestSize int) (string, error) {
	salt, rounds, explicitRounds, err := parseCryptSalt(raw, prefix, 16, true)
	if err != nil {
		return "", err
	}
	key := []byte(password)
	keyLen, saltLen := len(key), len(salt)
	h := newHash()

	h.Write(key)
	h.Write(salt)
	h.Write(key)
	sumB := h.Sum(nil)

	h.Reset()
	h.Write(key)
	h.Write(salt)
	h.Write(repeatByteSequence(sumB, keyLen))
	for i := keyLen; i > 0; i >>= 1 {
		if i%2 == 0 {
			h.Write(key)
		} else {
			h.Write(sumB)
		}
	}
	sumA := h.Sum(nil)

	h.Reset()
	for i := 0; i < keyLen; i++ {
		h.Write(key)
	}
	seqP := repeatByteSequence(h.Sum(nil), keyLen)

	h.Reset()
	for i := 0; i < 16+int(sumA[0]); i++ {
		h.Write(salt)
	}
	seqS := repeatByteSequence(h.Sum(nil), saltLen)

	for i := 0; i < rounds; i++ {
		h.Reset()
		if i&1 != 0 {
			h.Write(seqP)
		} else {
			h.Write(sumA)
		}
		if i%3 != 0 {
			h.Write(seqS)
		}
		if i%7 != 0 {
			h.Write(seqP)
		}
		if i&1 != 0 {
			h.Write(sumA)
		} else {
			h.Write(seqP)
		}
		copy(sumA, h.Sum(nil))
	}

	var order []byte
	switch digestSize {
	case 32:
		order = []byte{
			sumA[20], sumA[10], sumA[0], sumA[11], sumA[1], sumA[21],
			sumA[2], sumA[22], sumA[12], sumA[23], sumA[13], sumA[3],
			sumA[14], sumA[4], sumA[24], sumA[5], sumA[25], sumA[15],
			sumA[26], sumA[16], sumA[6], sumA[17], sumA[7], sumA[27],
			sumA[8], sumA[28], sumA[18], sumA[29], sumA[19], sumA[9],
			sumA[30], sumA[31],
		}
	case 64:
		order = []byte{
			sumA[42], sumA[21], sumA[0], sumA[1], sumA[43], sumA[22],
			sumA[23], sumA[2], sumA[44], sumA[45], sumA[24], sumA[3],
			sumA[4], sumA[46], sumA[25], sumA[26], sumA[5], sumA[47],
			sumA[48], sumA[27], sumA[6], sumA[7], sumA[49], sumA[28],
			sumA[29], sumA[8], sumA[50], sumA[51], sumA[30], sumA[9],
			sumA[10], sumA[52], sumA[31], sumA[32], sumA[11], sumA[53],
			sumA[54], sumA[33], sumA[12], sumA[13], sumA[55], sumA[34],
			sumA[35], sumA[14], sumA[56], sumA[57], sumA[36], sumA[15],
			sumA[16], sumA[58], sumA[37], sumA[38], sumA[17], sumA[59],
			sumA[60], sumA[39], sumA[18], sumA[19], sumA[61], sumA[40],
			sumA[41], sumA[20], sumA[62], sumA[63],
		}
	default:
		return "", errors.New("unsupported SHA crypt digest")
	}

	var buf bytes.Buffer
	buf.WriteString(prefix)
	if explicitRounds {
		buf.WriteString("rounds=")
		buf.WriteString(strconv.Itoa(rounds))
		buf.WriteByte('$')
	}
	buf.Write(salt)
	buf.WriteByte('$')
	buf.Write(cryptBase64(order))
	return buf.String(), nil
}

func md5Crypt(password, raw string) (string, error) {
	salt, _, _, err := parseCryptSalt(raw, "$1$", 8, false)
	if err != nil {
		return "", err
	}
	key := []byte(password)
	keyLen := len(key)
	h := md5.New()

	h.Write(key)
	h.Write(salt)
	h.Write(key)
	sumB := h.Sum(nil)

	h.Reset()
	h.Write(key)
	h.Write([]byte("$1$"))
	h.Write(salt)
	h.Write(repeatByteSequence(sumB, keyLen))
	for i := keyLen; i > 0; i >>= 1 {
		if i%2 == 0 {
			if len(key) > 0 {
				h.Write(key[:1])
			}
		} else {
			h.Write([]byte{0})
		}
	}
	sumA := h.Sum(nil)

	for i := 0; i < 1000; i++ {
		h.Reset()
		if i%2 != 0 {
			h.Write(key)
		} else {
			h.Write(sumA)
		}
		if i%3 != 0 {
			h.Write(salt)
		}
		if i%7 != 0 {
			h.Write(key)
		}
		if i&1 != 0 {
			h.Write(sumA)
		} else {
			h.Write(key)
		}
		copy(sumA, h.Sum(nil))
	}

	order := []byte{
		sumA[12], sumA[6], sumA[0], sumA[13], sumA[7], sumA[1],
		sumA[14], sumA[8], sumA[2], sumA[15], sumA[9], sumA[3],
		sumA[5], sumA[10], sumA[4], sumA[11],
	}
	var buf bytes.Buffer
	buf.WriteString("$1$")
	buf.Write(salt)
	buf.WriteByte('$')
	buf.Write(cryptBase64(order))
	return buf.String(), nil
}
