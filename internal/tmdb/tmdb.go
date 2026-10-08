// Package tmdb — TMDB 搜索与条目构建（对应原 lib/tmdb.js）
package tmdb

import (
	"encoding/json"
	"math"
	"net/url"
	"strconv"
	"strings"

	"github.com/syuim/vps-widget/internal/util"
)

const (
	tmdbSearchTV    = "https://api.themoviedb.org/3/search/tv"
	tmdbSearchMovie = "https://api.themoviedb.org/3/search/movie"
	tmdbDetailTV    = "https://api.themoviedb.org/3/tv/"
)

// GenreMap TMDB 类型 ID 中文名
var GenreMap = map[int]string{
	28: "动作", 12: "冒险", 16: "动画", 35: "喜剧", 80: "犯罪", 99: "纪录片", 18: "剧情",
	10751: "家庭", 14: "奇幻", 36: "历史", 27: "恐怖", 10402: "音乐", 9648: "悬疑",
	10749: "爱情", 878: "科幻", 10770: "电视电影", 53: "惊悚", 10752: "战争", 37: "西部",
	10759: "动作冒险", 10762: "儿童", 10763: "新闻", 10764: "真人秀", 10765: "科幻奇幻",
	10766: "肥皂剧", 10767: "脱口秀", 10768: "战争政治",
}

// CountryMap 国家/地区码中文名
var CountryMap = map[string]string{
	"CN": "中国大陆", "JP": "日本", "KR": "韩国", "US": "美国",
	"GB": "英国", "TW": "中国台湾", "HK": "中国香港",
}

// Result TMDB 搜索结果的原始条目
type Result struct {
	ID            int      `json:"id"`
	Name          string   `json:"name"`
	Title         string   `json:"title"`
	OriginalName  string   `json:"original_name"`
	FirstAirDate  string   `json:"first_air_date"`
	ReleaseDate   string   `json:"release_date"`
	PosterPath    *string  `json:"poster_path"`
	BackdropPath  *string  `json:"backdrop_path"`
	Overview      string   `json:"overview"`
	VoteAverage   float64  `json:"vote_average"`
	VoteCount     int      `json:"vote_count"`
	Popularity    float64  `json:"popularity"`
	GenreIDs      []int    `json:"genre_ids"`
	OriginCountry []string `json:"origin_country"`
}

// Info 榜单条目（各源共用的增强字段，字段顺序与原 JS 对象一致）
type Info struct {
	ID             string  `json:"id"`
	TMDBID         int     `json:"tmdbId"`
	Type           string  `json:"type"`
	MediaType      string  `json:"mediaType"`
	Title          string  `json:"title"`
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

// authParams v4 token 走 Bearer 头，否则用 api_key 参数（对应原 authParams）
func authParams(apiKey string, p url.Values) map[string]string {
	headers := map[string]string{"accept": "application/json"}
	if strings.HasPrefix(apiKey, "eyJ") {
		headers["Authorization"] = "Bearer " + apiKey
	} else {
		p.Set("api_key", apiKey)
	}
	return headers
}

// Search 底层搜索，返回 TMDB results 原始数组（失败返回 nil）
func Search(query, apiKey string, isMovie bool, year string) []Result {
	base := tmdbSearchTV
	if isMovie {
		base = tmdbSearchMovie
	}
	p := url.Values{}
	p.Set("query", query)
	p.Set("language", "zh-CN")
	headers := authParams(apiKey, p)
	if year != "" {
		if isMovie {
			p.Set("primary_release_year", year)
		} else {
			p.Set("first_air_date_year", year)
		}
	}
	r, err := util.Get(base+"?"+p.Encode(), headers)
	if err != nil || r.Status != 200 {
		return nil
	}
	var resp struct {
		Results []Result `json:"results"`
	}
	if json.Unmarshal([]byte(r.Body), &resp) != nil {
		return nil
	}
	return resp.Results
}

// FetchLastAirDate 拿 tv 详情的最新播出日期（失败返回空串）
func FetchLastAirDate(tvID int, apiKey string) string {
	p := url.Values{}
	p.Set("language", "zh-CN")
	headers := authParams(apiKey, p)
	r, err := util.Get(tmdbDetailTV+strconv.Itoa(tvID)+"?"+p.Encode(), headers)
	if err != nil || r.Status != 200 {
		return ""
	}
	var resp struct {
		LastAirDate string `json:"last_air_date"`
	}
	if json.Unmarshal([]byte(r.Body), &resp) != nil {
		return ""
	}
	return resp.LastAirDate
}

// BuildInfo 把一条 TMDB 结果构造成榜单条目（对应原 buildInfo）
func BuildInfo(res Result, mediaType string) Info {
	isMovie := res.Title != "" // TMDB: movie 结果用 title，tv 用 name
	mType := mediaType
	if mType == "" {
		if isMovie {
			mType = "movie"
		} else {
			mType = "tv"
		}
	}
	fa := res.FirstAirDate
	if fa == "" {
		fa = res.ReleaseDate
	}
	genres := []string{}
	for _, g := range res.GenreIDs {
		if name, ok := GenreMap[g]; ok {
			genres = append(genres, name)
		}
	}
	if len(genres) > 3 {
		genres = genres[:3]
	}
	region := ""
	for i, c := range res.OriginCountry {
		if i > 0 {
			region += "/"
		}
		if name, ok := CountryMap[c]; ok {
			region += name
		} else {
			region += c
		}
	}
	if region == "" {
		region = "未知地区"
	}
	score := math.Round(res.VoteAverage*10) / 10
	title := res.Name
	if title == "" {
		title = res.Title
	}
	return Info{
		ID:             strconv.Itoa(res.ID),
		TMDBID:         res.ID,
		Type:           "tmdb",
		MediaType:      mType,
		Title:          title,
		Description:    res.Overview,
		Rating:         score,
		VoteCount:      res.VoteCount,
		Popularity:     res.Popularity,
		ReleaseDate:    fa,
		LastUpdateDate: fa,
		PosterPath:     res.PosterPath,
		BackdropPath:   res.BackdropPath,
		GenreTitle:     strings.Join(genres, ","),
		RegionTitle:    region,
	}
}

// StrictMatch 严格标题+年份匹配，返回第一条命中（需双图）
// requireGenre 为 0 表示不限定；notYetAired 为 true 时拦截未开播
func StrictMatch(results []Result, title, year string, requireGenre int, notYetAired bool) *Result {
	target := strings.ToLower(title)
	today := util.TodayString()
	for i := range results {
		res := &results[i]
		n := strings.ToLower(res.Name)
		o := strings.ToLower(res.OriginalName)
		if !(strings.Contains(n, target) || strings.Contains(o, target) || strings.Contains(target, n)) {
			continue
		}
		fa := res.FirstAirDate
		if fa == "" {
			fa = res.ReleaseDate
		}
		if year != "" && fa != "" && !strings.HasPrefix(fa, year) {
			continue
		}
		if res.PosterPath == nil || res.BackdropPath == nil {
			continue
		}
		if requireGenre != 0 && !containsInt(res.GenreIDs, requireGenre) {
			continue
		}
		if notYetAired && (fa == "" || fa > today) {
			continue
		}
		return res
	}
	return nil
}

func containsInt(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
