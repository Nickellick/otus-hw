package hw02unpackstring

import (
	"errors"
	"strconv"
	"strings"
	"unicode"
)

var ErrInvalidString = errors.New("invalid string")

func Unpack(input string) (string, error) {
	builder := &strings.Builder{}
	isUnpacking := true
	var lastRune rune

	for _, char := range input {
		if !unicode.IsDigit(char) {
			if lastRune != 0 {
				builder.WriteRune(lastRune)
			}
			isUnpacking = false
			lastRune = char
			continue
		}
		if isUnpacking {
			return "", ErrInvalidString
		}
		isUnpacking = true
		repeats, err := strconv.Atoi(string(char))
		if err != nil {
			return "", err
		}
		for _, char := range strings.Repeat(string(lastRune), repeats) {
			builder.WriteRune(char)
		}
		lastRune = 0
	}
	if lastRune != 0 {
		builder.WriteRune(lastRune)
	}
	return builder.String(), nil
}
