// mgtv 芒果TV热榜（剧集/综艺，对应原 lib/mgtv.js）
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

const mgtvAPI = "https://pianku.api.mgtv.com/rider/list/pcweb/v3"

var mgtvHeaders = map[string]string{
	"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36",
	"Referer":    "https://www.mgtv.com/",
}

var mgtvRegions = []struct {
	Value     string
	ChannelID int
	Limit     int
	Params    url.Values
}{
	{"tv", 2, 150, url.Values{"kind": {"a1"}, "area": {"a1"}, "year": {"all"}, "sort": {"c1"}, "chargeInfo": {"a1"}}},
	{"show", 1, 150, url.Values{"kind": {"a1"}, "area": {"a1"}, "year": {"all"}, "sort": {"c1"}}},
}

var reMgtvYear = regexp.MustCompile(`\b(20\d{2})\b`)

func mgtvYearFrom(text string) string {
	m := reMgtvYear.FindStringSubmatch(text)
	if m == nil {
		return ""
	}
	return m[1]
}

func mgtvFetchRegion(regionValue string, channelID, limit int, params url.Values) []doubanItem {
	const pageSize = 30
	items := []doubanItem{}
	pageCount := (limit + pageSize - 1) / pageSize
	for p := 1; p <= pageCount; p++ {
		q := url.Values{}
		q.Set("allowedpn", "1")
		q.Set("channelId", strconv.Itoa(channelID))
		q.Set("pn", strconv.Itoa(p))
		q.Set("pc", strconv.Itoa(pageSize))
		for k, vs := range params {
			q[k] = vs
		}
		r, err := util.Get(mgtvAPI+"?"+q.Encode(), mgtvHeaders)
		if err != nil || r.Status != 200 {
			break
		}
		var resp struct {
			Data struct {
				HitDocs []struct {
					Title    string `json:"title"`
					Subtitle string `json:"subtitle"`
				} `json:"hitDocs"`
			} `json:"data"`
		}
		if json.Unmarshal([]byte(r.Body), &resp) != nil {
			break
		}
		docs := resp.Data.HitDocs
		if len(docs) == 0 {
			break
		}
		for _, d := range docs {
			items = append(items, doubanItem{Title: d.Title, CardSubtitle: d.Subtitle})
		}
		if len(items) >= limit {
			items = items[:limit]
			break
		}
	}
	return items
}

func mgtvMatchOne(item doubanItem, apiKey string) *tmdb.Info {
	dbTitle := util.CleanMgtv(item.Title)
	year := mgtvYearFrom(item.CardSubtitle)
	if year == "" {
		year = mgtvYearFrom(item.Title)
	}
	if year != "" {
		dbTitle = strings.TrimSpace(strings.Replace(dbTitle, year, "", 1))
	}
	results := tmdb.Search(dbTitle, apiKey, false, year)
	res := matchTVResult(results, dbTitle, year, false)
	if res == nil {
		return nil
	}
	info := withLastAirDate(res, apiKey)
	return &info
}

func mgtvFetch(apiKey string) (any, error) {
	out := util.NewOMap()
	out.Set("last_updated", util.BjStamp())
	for _, region := range mgtvRegions {
		items := mgtvFetchRegion(region.Value, region.ChannelID, region.Limit, region.Params)
		matched := []tmdb.Info{}
		for _, item := range items {
			if info := mgtvMatchOne(item, apiKey); info != nil {
				matched = append(matched, *info)
			}
		}
		out.Set(region.Value, matched)
	}
	return out, nil
}
