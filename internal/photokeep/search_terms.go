package photokeep

import (
	"strings"
	"sync"
	"unicode"

	"github.com/sukitss/thai-nlp-go/tokenize"
)

var (
	searchSegmenterOnce sync.Once
	searchSegmenter     *tokenize.Segmenter
)

func searchWords(text string) []string {
	searchSegmenterOnce.Do(func() {
		searchSegmenter, _ = tokenize.NewDefault()
	})
	if searchSegmenter == nil {
		return strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) && !unicode.IsMark(r) })
	}

	words := searchSegmenter.SegmentNoWS(strings.ToLower(text))
	clean := make([]string, 0, len(words))
	for _, word := range words {
		for _, part := range strings.FieldsFunc(word, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsNumber(r) && !unicode.IsMark(r)
		}) {
			clean = append(clean, part)
		}
	}
	return clean
}

func searchWordGroups(words []string) [][]string {
	groups := make([][]string, 0, len(words))
	for _, word := range words {
		groups = append(groups, []string{word})
	}
	return groups
}

func uniqueSearchWords(text string) []string {
	seen := make(map[string]bool)
	words := make([]string, 0)
	for _, word := range searchWords(text) {
		if !seen[word] {
			seen[word] = true
			words = append(words, word)
		}
	}
	return words
}

func searchWordSet(text string) map[string]bool {
	words := make(map[string]bool)
	for _, word := range searchWords(text) {
		words[word] = true
	}
	return words
}

func searchTagSet(tags string) map[string]bool {
	words := make(map[string]bool)
	for _, tag := range strings.Split(tags, ",") {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag != "" {
			words[tag] = true
		}
		for _, word := range searchWords(tag) {
			words[word] = true
		}
	}
	return words
}

func searchRank(photo Photo, groups [][]string, query string) (float64, int) {
	filename := searchWordSet(photo.Filename)
	description := searchWordSet(photo.Description)
	tags := searchTagSet(photo.Tags)
	score, matched := 0.0, 0
	for _, group := range groups {
		best := 0.0
		for index, word := range group {
			weight := 1.0
			if index > 0 {
				weight = 0.75
			}
			candidate := 0.0
			if tags[word] {
				candidate += 5
			}
			if filename[word] {
				candidate += 3
			}
			if description[word] {
				candidate += 2
			}
			if candidate*weight > best {
				best = candidate * weight
			}
		}
		if best > 0 {
			score += best
			matched++
		}
	}
	phrase := strings.ToLower(strings.TrimSpace(query))
	if phrase != "" {
		for _, tag := range strings.Split(photo.Tags, ",") {
			if strings.ToLower(strings.TrimSpace(tag)) == phrase {
				score += 12
				break
			}
		}
		if strings.Contains(strings.ToLower(photo.Filename), phrase) {
			score += 8
		}
		if strings.Contains(strings.ToLower(photo.Description), phrase) {
			score += 6
		}
	}
	return score, matched
}

func searchLikePattern(word string) string {
	word = strings.ReplaceAll(word, "!", "!!")
	word = strings.ReplaceAll(word, "%", "!%")
	word = strings.ReplaceAll(word, "_", "!_")
	return "%" + word + "%"
}
