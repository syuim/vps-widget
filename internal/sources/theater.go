// theater 剧场平台（豆瓣片单，15 个剧场，对应原 lib/theater.js）
package sources

import (
	"fmt"
	"regexp"
	"sort"

	"github.com/syuim/vps-widget/internal/tmdb"
	"github.com/syuim/vps-widget/internal/util"
)

var theaterList = []struct{ Name, ID string }{
	{"迷雾剧场", "128396349"}, {"白夜剧场", "158539495"},
	{"X剧场", "155026800"}, {"玛卡巴卡的悬疑剧", "160885987"},
	{"横屏短剧", "152299516"}, {"生花剧场", "159069554"},
	{"大家剧场", "160644809"}, {"小逗剧场", "146055365"},
	{"十分剧场", "147708618"}, {"板凳单元", "163392459"},
	{"萤火单元", "163549603"}, {"正午阳光", "125370543"},
	{"恋恋剧场", "156086548"}, {"悬疑剧场", "128400108"},
	{"微尘剧场", "161658331"},
}

const theaterPageSize = 25

var theaterHeaders = map[string]string{
	"User-Agent": "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
}

var (
	reDoulistUL    = regexp.MustCompile(`(?i)<ul[^>]*class="[^"]*doulist-items[^"]*"[^>]*>([\s\S]*?)</ul>`)
	reDoulistLI    = regexp.MustCompile(`(?i)<li>([\s\S]*?)</li>`)
	reDoulistTitle = regexp.MustCompile(`(?i)<h2[^>]*class="[^"]*title[^"]*"[^>]*>([\s\S]*?)</h2>`)
	reDoulistMeta  = regexp.MustCompile(`(?i)<div[^>]*class="[^"]*meta[^"]*"[^>]*>([\s\S]*?)</div>`)
	reTheaterYear  = regexp.MustCompile(`(\d{4})`)
)

type theaterRaw struct {
	Title string
	Year  string
}

// parseDoulistPage 解析豆瓣片单页：定位 doulist-items 列表，提取 .title 与 .meta 里的标题和年份
func parseDoulistPage(html string) []theaterRaw {
	items := []theaterRaw{}
	ulM := reDoulistUL.FindStringSubmatch(html)
	if ulM == nil {
		return items
	}
	block := ulM[1]
	for _, m := range reDoulistLI.FindAllStringSubmatch(block, -1) {
		it := m[1]
		titleM := reDoulistTitle.FindStringSubmatch(it)
		if titleM == nil {
			continue
		}
		title := stripTagsCollapse(titleM[1])
		if title == "" {
			continue
		}
		metaText := ""
		if metaM := reDoulistMeta.FindStringSubmatch(it); metaM != nil {
			metaText = stripTagsTrim(metaM[1])
		}
		year := ""
		if ym := reTheaterYear.FindStringSubmatch(metaText); ym != nil {
			year = ym[1]
		}
		items = append(items, theaterRaw{Title: util.CleanDouban(title), Year: year})
	}
	return items
}

func theaterFetchOne(id string) ([]theaterRaw, int) {
	items := []theaterRaw{}
	pages := 0
	start := 0
	for {
		pages++
		u := fmt.Sprintf("https://m.douban.com/doulist/%s/?start=%d", id, start)
		r, err := util.Get(u, theaterHeaders)
		if err != nil {
			break
		}
		if r.Status != 200 {
			break
		}
		pageItems := parseDoulistPage(r.Body)
		if len(pageItems) == 0 {
			break
		}
		items = append(items, pageItems...)
		if len(pageItems) < theaterPageSize {
			break
		}
		start += theaterPageSize
	}
	return items, pages
}

func theaterMatchOne(item theaterRaw, apiKey string) *tmdb.Info {
	results := tmdb.Search(item.Title, apiKey, false, item.Year)
	res := matchTVResult(results, item.Title, item.Year, true)
	if res == nil {
		return nil
	}
	info := withLastAirDate(res, apiKey)
	return &info
}

func theaterFetch(apiKey string) (any, error) {
	out := util.NewOMap()
	out.Set("last_updated", util.BjStamp())
	for _, t := range theaterList {
		items, pages := theaterFetchOne(t.ID)
		aired := []tmdb.Info{}
		for _, item := range items {
			if info := theaterMatchOne(item, apiKey); info != nil {
				aired = append(aired, *info)
			}
		}
		sort.Slice(aired, func(i, j int) bool {
			return aired[i].ReleaseDate > aired[j].ReleaseDate
		})
		entry := util.NewOMap()
		entry.Set("aired", aired)
		entry.Set("upcoming", []tmdb.Info{})
		entry.Set("totalItems", len(items))
		entry.Set("totalPages", pages)
		out.Set(t.Name, entry)
	}
	return out, nil
}
