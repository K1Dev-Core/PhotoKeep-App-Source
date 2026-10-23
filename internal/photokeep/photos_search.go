package photokeep

import (
	"sort"
	"strings"
)

func (a *Service) Search(query, tag string) (SearchResult, error) {
	return a.SearchPage(query, tag, 0, 0)
}

func (a *Service) SearchPage(query, tag string, offset, limit int) (SearchResult, error) {
	query = strings.TrimSpace(query)
	if query != "" {
		return a.searchPageRanked(query, tag, offset, limit)
	}
	db := a.db.Model(&Photo{})
	if tag = strings.TrimSpace(tag); tag != "" {
		db = db.Where("(',' || LOWER(tags) || ',') LIKE ?", "%,"+strings.ToLower(tag)+",%")
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return SearchResult{}, err
	}
	var photos []Photo
	db = db.Order("backed_up_at desc, id desc")
	page := db.Offset(maxInt(0, offset))
	if limit > 0 {
		page = page.Limit(limit)
	}
	if err := page.Find(&photos).Error; err != nil {
		return SearchResult{}, err
	}
	tags, err := a.tagCloud()
	if err != nil {
		return SearchResult{}, err
	}
	return SearchResult{Photos: photos, Total: total, Tags: tags}, nil
}

type searchHit struct {
	ID      uint
	Matched int
	Score   float64
	Backed  int64
}

func (a *Service) searchPageRanked(query, tag string, offset, limit int) (SearchResult, error) {
	words := uniqueSearchWords(query)
	if len(words) == 0 {
		return SearchResult{Photos: []Photo{}, Tags: []TagCount{}}, nil
	}
	groups := searchWordGroups(words)

	var filters []string
	var args []interface{}
	for _, group := range groups {
		alternatives := make([]string, 0, len(group))
		for _, word := range group {
			pattern := searchLikePattern(word)
			if len([]rune(word)) >= 3 && !strings.ContainsAny(word, "%_") {
				alternatives = append(alternatives, "id IN (SELECT rowid FROM photo_search WHERE filename LIKE ? OR description LIKE ? OR tags LIKE ?)")
			} else {
				alternatives = append(alternatives, "(filename LIKE ? ESCAPE '!' OR description LIKE ? ESCAPE '!' OR tags LIKE ? ESCAPE '!')")
			}
			args = append(args, pattern, pattern, pattern)
		}
		filters = append(filters, "("+strings.Join(alternatives, " OR ")+")")
	}
	db := a.db.Model(&Photo{}).Select("id", "filename", "description", "tags", "backed_up_at").Where("("+strings.Join(filters, " OR ")+")", args...)
	if tag = strings.TrimSpace(tag); tag != "" {
		db = db.Where("(',' || LOWER(tags) || ',') LIKE ?", "%,"+strings.ToLower(tag)+",%")
	}
	var candidates []Photo
	if err := db.Find(&candidates).Error; err != nil {
		return SearchResult{}, err
	}
	hits := make([]searchHit, 0, len(candidates))
	for _, photo := range candidates {
		score, matched := searchRank(photo, groups, query)
		if matched == 0 {
			continue
		}
		hits = append(hits, searchHit{ID: photo.ID, Matched: matched, Score: score, Backed: photo.BackedUpAt.UnixNano()})
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Matched != hits[j].Matched {
			return hits[i].Matched > hits[j].Matched
		}
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		if hits[i].Backed != hits[j].Backed {
			return hits[i].Backed > hits[j].Backed
		}
		return hits[i].ID > hits[j].ID
	})
	total := int64(len(hits))
	start := minInt(maxInt(0, offset), len(hits))
	end := len(hits)
	if limit > 0 {
		end = minInt(end, start+limit)
	}
	ids := make([]uint, 0, end-start)
	for _, hit := range hits[start:end] {
		ids = append(ids, hit.ID)
	}
	page := make([]Photo, 0, len(ids))
	if len(ids) > 0 {
		var fetched []Photo
		if err := a.db.Where("id IN ?", ids).Find(&fetched).Error; err != nil {
			return SearchResult{}, err
		}
		byID := make(map[uint]Photo, len(fetched))
		for _, photo := range fetched {
			byID[photo.ID] = photo
		}
		for _, id := range ids {
			if photo, ok := byID[id]; ok {
				page = append(page, photo)
			}
		}
	}
	tags, err := a.tagCloud()
	if err != nil {
		return SearchResult{}, err
	}
	return SearchResult{Photos: page, Total: total, Tags: tags}, nil
}

func (a *Service) tagCloud() ([]TagCount, error) {
	a.tagCacheMu.RLock()
	if a.tagCacheValid {
		cached := append([]TagCount(nil), a.tagCache...)
		a.tagCacheMu.RUnlock()
		return cached, nil
	}
	version := a.tagCacheVersion
	a.tagCacheMu.RUnlock()

	var rows []Photo
	if err := a.db.Select("tags").Find(&rows).Error; err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, row := range rows {
		for _, t := range strings.Split(row.Tags, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				counts[t]++
			}
		}
	}
	out := make([]TagCount, 0, len(counts))
	for name, count := range counts {
		out = append(out, TagCount{name, count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Name < out[j].Name
		}
		return out[i].Count > out[j].Count
	})
	a.tagCacheMu.Lock()
	if a.tagCacheVersion == version {
		a.tagCache = append([]TagCount(nil), out...)
		a.tagCacheValid = true
	}
	a.tagCacheMu.Unlock()
	return out, nil
}

func (a *Service) invalidateTagCloud() {
	a.tagCacheMu.Lock()
	a.tagCacheVersion++
	a.tagCacheValid = false
	a.tagCache = nil
	a.tagCacheMu.Unlock()
}
