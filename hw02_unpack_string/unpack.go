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
	var lastRune rune
	hasLastRune := false

	for _, char := range input {
		if !unicode.IsDigit(char) {
			if hasLastRune {
				builder.WriteRune(lastRune)
			}
			lastRune = char
			hasLastRune = true
			continue
		}
		if !hasLastRune {
			return "", ErrInvalidString
		}
		repeats, err := strconv.Atoi(string(char))
		if err != nil {
			return "", err
		}
		for i := 0; i < repeats; i++ {
			builder.WriteRune(lastRune)
		}
		hasLastRune = false
	}
	if hasLastRune {
		builder.WriteRune(lastRune)
	}
	return builder.String(), nil
}
