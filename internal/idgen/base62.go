package idgen

import (
	"errors"
	"hash/fnv"
	"math/rand"
	"strings"
)

const DefaultAlphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func ShuffleAlphabet(seed string) string {
	if seed == "" {
		return DefaultAlphabet
	}

	h := fnv.New64a()
	_, _ = h.Write([]byte(seed))
	seedInt := int64(h.Sum64())

	r := rand.New(rand.NewSource(seedInt))
	chars := []byte(DefaultAlphabet)

	for i := len(chars) - 1; i > 0; i-- {
		j := r.Intn(i + 1)
		chars[i], chars[j] = chars[j], chars[i]
	}

	return string(chars)
}

type Base62Encoder struct {
	alphabet  string
	charIndex map[byte]uint64
	minPadLen int
}

func NewBase62Encoder(alphabet string, minPadLen int) (*Base62Encoder, error) {
	if alphabet == "" {
		alphabet = DefaultAlphabet
	}
	if len(alphabet) != 62 {
		return nil, errors.New("o alfabeto Base62 deve conter exatamente 62 caracteres únicos")
	}

	charIndex := make(map[byte]uint64, 62)
	for i := 0; i < len(alphabet); i++ {
		b := alphabet[i]
		if _, exists := charIndex[b]; exists {
			return nil, errors.New("caracteres duplicados encontrados no alfabeto Base62")
		}
		charIndex[b] = uint64(i)
	}

	return &Base62Encoder{
		alphabet:  alphabet,
		charIndex: charIndex,
		minPadLen: minPadLen,
	}, nil
}

func NewBase62EncoderWithSeed(seed string, minPadLen int) (*Base62Encoder, error) {
	alphabet := ShuffleAlphabet(seed)
	return NewBase62Encoder(alphabet, minPadLen)
}

func (b *Base62Encoder) Encode(num uint64) string {
	return b.EncodeWithPad(num, b.minPadLen)
}

func (b *Base62Encoder) EncodeWithPad(num uint64, minPadLen int) string {
	if num == 0 {
		pad := string(b.alphabet[0])
		if minPadLen > 1 {
			return strings.Repeat(pad, minPadLen)
		}
		return pad
	}

	var sb strings.Builder
	base := uint64(len(b.alphabet))

	for num > 0 {
		remainder := num % base
		sb.WriteByte(b.alphabet[remainder])
		num /= base
	}

	runes := []byte(sb.String())

	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}

	encoded := string(runes)
	if len(encoded) < minPadLen {
		padCount := minPadLen - len(encoded)
		encoded = strings.Repeat(string(b.alphabet[0]), padCount) + encoded
	}

	return encoded
}

func (b *Base62Encoder) Decode(str string) (uint64, error) {
	if len(str) == 0 {
		return 0, errors.New("string vazia")
	}

	base := uint64(len(b.alphabet))
	var result uint64

	for i := 0; i < len(str); i++ {
		val, exists := b.charIndex[str[i]]
		if !exists {
			return 0, errors.New("caractere inválido na string Base62")
		}
		result = result*base + val
	}

	return result, nil
}
