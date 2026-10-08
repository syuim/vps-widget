// bangumi 番剧榜（bgm.tv 排名榜 + TMDB 动画匹配，对应原 lib/bangumi.js）
package sources

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/syuim/vps-widget/internal/tmdb"
	"github.com/syuim/vps-widget/internal/util"
)

const (
	bgmBase     = "https://bgm.tv/anime/browser?sort=rank" // 公开排名榜（sort=collects 需登录）
	bgmMaxPages = 2                                        // 抓取页数
	bgmMaxItems = 40                                       // 最多匹配条数
	bgmUA       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

var (
	reBgmLI      = regexp.MustCompile(`(?i)<li[^>]*class="item[^"]*"[^>]*>([\s\S]*?)</li>`)
	reBgmTitle   = regexp.MustCompile(`(?i)<a[^>]*class="[^"]*\bl\b[^"]*"[^>]*>([\s\S]*?)</a>`)
	reBgmOrig    = regexp.MustCompile(`(?i)<small[^>]*class="grey"[^>]*>([\s\S]*?)</small>`)
	reBgmInfo    = regexp.MustCompile(`(?i)<p[^>]*class="info[^"]*"[^>]*>([\s\S]*?)</p>`)
	reBgmYearCN  = regexp.MustCompile(`(\d{4})年`)
	reBgmYearRaw = regexp.MustCompile(`\b(19|20)\d{2}\b`)
)

type bgmRaw struct {
	Title         string
	OriginalTitle string
	Year          string
}

// bangumiItem 输出条目（对应原 buildInfo + bangumi build 附加字段，字段顺序一致）
type bangumiItem struct {
	ID             string   `json:"id"`
	TMDBID         int      `json:"tmdbId"`
	Type           string   `json:"type"`
	MediaType      string   `json:"mediaType"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Rating         float64  `json:"rating"`
	VoteCount      int      `json:"voteCount"`
	Popularity     float64  `json:"popularity"`
	ReleaseDate    string   `json:"releaseDate"`
	LastUpdateDate string   `json:"lastUpdateDate"`
	PosterPath     *string  `json:"posterPath"`
	BackdropPath   *string  `json:"backdropPath"`
	GenreTitle     string   `json:"genreTitle"`
	RegionTitle    string   `json:"regionTitle"`
	RawGenres      []int    `json:"rawGenres"`
	RawCountries   []string `json:"rawCountries"`
}

type bangumiData struct {
	LastUpdated  string         `json:"last_updated"`
	TotalMatched int            `json:"total_matched"`
	HotAnime     []*bangumiItem `json:"hot_anime"`
}

func parseBgmPage(html string) []bgmRaw {
	items := []bgmRaw{}
	for _, m := range reBgmLI.FindAllStringSubmatch(html, -1) {
		block := m[1]
		tm := reBgmTitle.FindStringSubmatch(block)
		if tm == nil {
			continue
		}
		titleCn := stripTagsCollapse(tm[1])
		if titleCn == "" {
			continue
		}
		orig := titleCn
		if om := reBgmOrig.FindStringSubmatch(block); om != nil {
			orig = stripTagsTrim(om[1])
		}
		infoText := ""
		if im := reBgmInfo.FindStringSubmatch(block); im != nil {
			infoText = stripTagsSpace(im[1])
		}
		year := ""
		if ym := reBgmYearCN.FindStringSubmatch(infoText); ym != nil {
			year = ym[1]
		} else if ym := reBgmYearRaw.FindStringSubmatch(infoText); ym != nil {
			// 与上游一致：fallback 正则捕获组 1 为 "19"/"20"
			year = ym[1]
		}
		items = append(items, bgmRaw{Title: titleCn, OriginalTitle: orig, Year: year})
	}
	return items
}

func bgmFetchRaw() []bgmRaw {
	items := []bgmRaw{}
	for p := 1; p <= bgmMaxPages; p++ {
		r, err := util.Get(fmt.Sprintf("%s&page=%d", bgmBase, p), map[string]string{
			"User-Agent":      bgmUA,
			"Accept-Language": "zh-CN,zh;q=0.9",
		})
		if err != nil || r.Status != 200 {
			continue
		}
		items = append(items, parseBgmPage(r.Body)...)
		if len(items) >= bgmMaxItems {
			break
		}
	}
	if len(items) > bgmMaxItems {
		items = items[:bgmMaxItems]
	}
	return items
}

func formatRating(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func bgmBuild(res tmdb.Result, mediaType string) *bangumiItem {
	base := tmdb.BuildInfo(res, mediaType)
	date4 := base.ReleaseDate
	if len(date4) > 4 {
		date4 = date4[:4]
	}
	overview := res.Overview
	if overview == "" {
		overview = "暂无简介"
	}
	base.Description = date4 + " · ⭐ " + formatRating(base.Rating) + " · " + base.RegionTitle + "\n" + overview
	genres := []string{}
	for _, g := range strings.Split(base.GenreTitle, ",") {
		if g != "动画" {
			genres = append(genres, g)
		}
	}
	genreTitle := strings.Join(genres, " / ")
	if genreTitle == "" {
		genreTitle = "动画"
	}
	rawGenres := res.GenreIDs
	if rawGenres == nil {
		rawGenres = []int{}
	}
	rawCountries := res.OriginCountry
	if rawCountries == nil {
		rawCountries = []string{}
	}
	return &bangumiItem{
		ID: base.ID, TMDBID: base.TMDBID, Type: base.Type, MediaType: base.MediaType,
		Title: base.Title, Description: base.Description, Rating: base.Rating,
		VoteCount: base.VoteCount, Popularity: base.Popularity,
		ReleaseDate: base.ReleaseDate, LastUpdateDate: base.LastUpdateDate,
		PosterPath: base.PosterPath, BackdropPath: base.BackdropPath,
		GenreTitle: genreTitle, RegionTitle: base.RegionTitle,
		RawGenres: rawGenres, RawCountries: rawCountries,
	}
}

func bgmMatchOne(title, year, apiKey string) *bangumiItem {
	// TV 优先
	if res := tmdb.StrictMatch(tmdb.Search(title, apiKey, false, year), title, year, 16, true); res != nil {
		return bgmBuild(*res, "tv")
	}
	// 可能是剧场版电影
	if res := tmdb.StrictMatch(tmdb.Search(title, apiKey, true, year), title, year, 16, true); res != nil {
		return bgmBuild(*res, "movie")
	}
	return nil
}

func bangumiFetch(apiKey string) (any, error) {
	raw := bgmFetchRaw()
	hot := []*bangumiItem{}
	for _, it := range raw {
		cn := util.CleanAnime(it.Title)
		orig := util.CleanAnime(it.OriginalTitle)
		info := bgmMatchOne(cn, it.Year, apiKey)
		if info == nil && orig != cn {
			info = bgmMatchOne(orig, it.Year, apiKey)
		}
		if info != nil {
			hot = append(hot, info)
		}
	}
	return &bangumiData{LastUpdated: util.BjStamp(), TotalMatched: len(hot), HotAnime: hot}, nil
}
