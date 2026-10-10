package hw10programoptimization

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type User struct {
	ID       int
	Name     string
	Username string
	Email    string
	Phone    string
	Password string
	Address  string
}

type DomainStat map[string]int

func GetDomainStat(r io.Reader, domain string) (DomainStat, error) {
	suffix := "." + strings.ToLower(domain)
	stat := make(DomainStat)
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Bytes()

		var user User
		if err := json.Unmarshal(line, &user); err != nil {
			return stat, err
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
