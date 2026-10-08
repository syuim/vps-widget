// douban 豆瓣热榜（9 个区域，对应原 lib/douban.js）
package sources

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/syuim/vps-widget/internal/tmdb"
	"github.com/syuim/vps-widget/internal/util"
)

const doubanAPI = "https://m.douban.com/rexxar/api/v2/subject/recent_hot/tv"

var doubanHeaders = map[string]string{
	"User-Agent": "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
	"Referer":    "https://m.douban.com/movie/",
}

var doubanRegions = []struct {
	Type  string
	Limit int
}{
	{"tv", 300},
	{"tv_domestic", 150},
	{"tv_american", 150},
	{"tv_japanese", 150},
	{"tv_korean", 150},
	{"tv_animation", 150},
	{"tv_documentary", 150},
	{"show_domestic", 150},
	{"show_foreign", 150},
}

var reYear4 = regexp.MustCompile(`^\d{4}$`)

type doubanItem struct {
	Title        string `json:"title"`
	CardSubtitle string `json:"card_subtitle"`
}

func doubanFetchRegion(regionType string, limit int) []doubanItem {
	q := url.Values{}
	q.Set("start", "0")
	q.Set("limit", strconv.Itoa(limit))
	q.Set("type", regionType)
	r, err := util.Get(doubanAPI+"?"+q.Encode(), doubanHeaders)
	if err != nil || r.Status != 200 {
		return nil
	}
	var resp struct {
		Items []doubanItem `json:"items"`
	}
	if json.Unmarshal([]byte(r.Body), &resp) != nil {
		return nil
	}
	return resp.Items
}

func doubanYearFromSubtitle(subtitle string) string {
	first := strings.TrimSpace(strings.Split(subtitle, "/")[0])
	if reYear4.MatchString(first) {
		return first
	}
	return ""
}

func doubanMatchOne(item doubanItem, apiKey string) *tmdb.Info {
	dbTitle := util.CleanDouban(item.Title)
	year := doubanYearFromSubtitle(item.CardSubtitle)
	results := tmdb.Search(dbTitle, apiKey, false, year)
	res := matchTVResult(results, dbTitle, year, false)
	if res == nil {
		return nil
	}
	info := withLastAirDate(res, apiKey)
	return &info
}

func doubanFetch(apiKey string) (any, error) {
	out := util.NewOMap()
	out.Set("last_updated", util.BjStamp())
	for _, region := range doubanRegions {
		items := doubanFetchRegion(region.Type, region.Limit)
		matched := []tmdb.Info{}
		for _, item := range items {
			if info := doubanMatchOne(item, apiKey); info != nil {
				matched = append(matched, *info)
			}
		}
		out.Set(region.Type, matched)
	}
	return out, nil
}
