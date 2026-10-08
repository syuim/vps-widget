// guduo 骨朵热度榜（剧集/综艺/动漫/电影 4 分类，对应原 lib/guduo.js）
package sources

import (
	"encoding/json"
	"fmt"

	"github.com/syuim/vps-widget/internal/tmdb"
	"github.com/syuim/vps-widget/internal/util"
)

const (
	guduoUA     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	guduoMaxTop = 30 // 每分类取前 30
)

func guduoURL(categoryCode, date string) string {
	return fmt.Sprintf("https://d2.guduomedia.com/m/v3/billboard/list?type=DAILY&category=%s&date=%s&attach=gdi&orderTitle=gdi&platformId=0", categoryCode, date)
}

type guduoRaw struct {
	Title    string
	Rank     int
	Heat     any
	Category string
}

// guduoItem 输出条目（journal 字段顺序与原 JS {...item, ...base} 展开一致）
type guduoItem struct {
	Title          string  `json:"title"`
	Rank           int     `json:"rank"`
	Heat           any     `json:"heat"`
	Category       string  `json:"category"`
	ID             string  `json:"id"`
	TMDBID         int     `json:"tmdbId"`
	Type           string  `json:"type"`
	MediaType      string  `json:"mediaType"`
	Description    string  `json:"description"`
	Rating         float64 `json:"rating"`
	VoteCount      int     `json:"voteCount"`
	Popularity     float64 `json:"popularity"`
	ReleaseDate    string  `json:"releaseDate"`
	LastUpdateDate string  `json:"lastUpdateDate"`
	PosterPath     *string `json:"posterPath"`
	BackdropPath   *string `json:"backdropPath"`
	GenreTitle     string  `json:"genreTitle"`
	RegionTitle    string  `json:"regionTitle"`
}

func guduoFetchCategory(category, url string) []guduoRaw {
	r, err := util.Get(url, map[string]string{"User-Agent": guduoUA})
	if err != nil || r.Status != 200 {
		return nil
	}
	var resp struct {
		Data []struct {
			Name string `json:"name"`
			GDI  any    `json:"gdi"`
		} `json:"data"`
	}
	if json.Unmarshal([]byte(r.Body), &resp) != nil {
		return nil
	}
	items := []guduoRaw{}
	for i, item := range resp.Data {
		if i >= guduoMaxTop {
			break
		}
		title := item.Name
		if title == "" {
			title = "未知名称"
		}
		var heat any
		if f, ok := item.GDI.(float64); ok {
			heat = f
		}
		if heat == nil {
			heat = 0
		}
		items = append(items, guduoRaw{Title: title, Rank: i + 1, Heat: heat, Category: category})
	}
	return items
}

func guduoFilterByGenre(results []tmdb.Result, genres ...int) []tmdb.Result {
	out := []tmdb.Result{}
	for _, r := range results {
		for _, g := range genres {
			hit := false
			for _, x := range r.GenreIDs {
				if x == g {
					hit = true
					break
				}
			}
			if hit {
				out = append(out, r)
				break
			}
		}
	}
	return out
}

func guduoMatchItem(item guduoRaw, apiKey string) *guduoItem {
	cleanT := util.CleanGuduo(item.Title)
	var best *tmdb.Result
	mediaType := "tv"

	switch item.Category {
	case "电影":
		res := tmdb.Search(cleanT, apiKey, true, "")
		if len(res) > 0 {
			best = &res[0]
		}
		mediaType = "movie"
	case "剧集":
		res := tmdb.Search(cleanT, apiKey, false, "")
		if len(res) == 0 {
			res = tmdb.Search(cleanT, apiKey, true, "")
			mediaType = "movie"
		}
		if len(res) > 0 {
			best = &res[0]
		}
	case "综艺":
		res := tmdb.Search(cleanT, apiKey, false, "")
		variety := guduoFilterByGenre(res, 10764, 10767)
		if len(variety) > 0 {
			best = &variety[0]
		} else if len(res) > 0 {
			best = &res[0]
		}
	case "动漫":
		res := tmdb.Search(cleanT, apiKey, false, "")
		anime := guduoFilterByGenre(res, 16)
		if len(anime) == 0 {
			res = tmdb.Search(cleanT, apiKey, true, "")
			anime = guduoFilterByGenre(res, 16)
		}
		if len(anime) > 0 {
			best = &anime[0]
		}
	}

	if best == nil {
		return nil
	}
	base := tmdb.BuildInfo(*best, mediaType)
	return &guduoItem{
		Title: base.Title, Rank: item.Rank, Heat: item.Heat, Category: item.Category,
		ID: base.ID, TMDBID: base.TMDBID, Type: base.Type, MediaType: base.MediaType,
		Description: base.Description, Rating: base.Rating, VoteCount: base.VoteCount,
		Popularity: base.Popularity, ReleaseDate: base.ReleaseDate, LastUpdateDate: base.LastUpdateDate,
		PosterPath: base.PosterPath, BackdropPath: base.BackdropPath,
		GenreTitle: base.GenreTitle, RegionTitle: base.RegionTitle,
	}
}

func guduoFetch(apiKey string) (any, error) {
	date := util.BjYesterday() // 骨朵数据只有前一天的
	cats := []struct{ name, code string }{
		{"剧集", "NETWORK_DRAMA"},
		{"综艺", "NETWORK_VARIETY"},
		{"动漫", "ALL_ANIME"},
		{"电影", "NETWORK_MOVIE"},
	}
	categories := util.NewOMap()
	for _, c := range cats {
		raw := guduoFetchCategory(c.name, guduoURL(c.code, date))
		matched := []*guduoItem{}
		for _, item := range raw {
			if info := guduoMatchItem(item, apiKey); info != nil {
				matched = append(matched, info)
			}
		}
		categories.Set(c.name, matched)
	}
	out := util.NewOMap()
	out.Set("source", "Guduo Media")
	out.Set("billboard_date", date)
	out.Set("last_updated", util.BjStamp())
	out.Set("categories", categories)
	return out, nil
}
