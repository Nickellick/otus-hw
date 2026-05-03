package hw03frequencyanalysis

import (
	"bufio"
	"sort"
	"strings"
)

type Pair struct {
	Key   string
	Value int
}

type PairList []Pair

func Top10(text string) []string {
	wordStats := make(map[string]int)
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := scanner.Text()
		words := strings.Fields(line)
		for _, word := range words {
			wordStats[word]++
		}
	}
	pl := make(PairList, 0, len(wordStats))
	for k, v := range wordStats {
		pl = append(pl, Pair{k, v})
	}

	sort.Slice(pl, func(i, j int) bool {
		if pl[i].Value == pl[j].Value {
			return pl[i].Key < pl[j].Key
		}
		return pl[i].Value > pl[j].Value
	})
	limit := 10
	if len(pl) < limit {
		limit = len(pl)
	}
	ranked := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		ranked = append(ranked, pl[i].Key)
	}
	return ranked
}
