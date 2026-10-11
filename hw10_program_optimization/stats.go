package hw10programoptimization

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type User struct {
	Email string
}

type DomainStat map[string]int

func GetDomainStat(r io.Reader, domain string) (DomainStat, error) {
	suffix := "." + strings.ToLower(domain)
	stat := make(DomainStat)
	scanner := bufio.NewScanner(r)
	// Делаем буфер в 1 МБ, изначально оставим 64 Кб по умолчанию
	scanner.Buffer(make([]byte, 64*1024*1024), 1024*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()

		var user User
		if err := json.Unmarshal(line, &user); err != nil {
			continue
		}

		_, localDomain, ok := strings.Cut(user.Email, "@")
		if !ok {
			continue
		}

		localDomain = strings.ToLower(localDomain)

		if strings.HasSuffix(localDomain, suffix) {
			stat[localDomain]++
		}
	}
	if err := scanner.Err(); err != nil {
		return stat, fmt.Errorf("scan users: %w", err)
	}
	return stat, nil
}
