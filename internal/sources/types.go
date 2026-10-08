// Package sources — 5 个榜单源抓取 + TMDB 匹配（对应原 lib/{guduo,douban,mgtv,theater,bangumi}.js）
package sources

import (
	"regexp"
	"strings"

	"github.com/syuim/vps-widget/internal/tmdb"
	"github.com/syuim/vps-widget/internal/util"
)

// Source 榜单源
type Source struct {
	Name  string
	Title string
	Fetch func(apiKey string) (any, error)
}

// All 全部数据源（顺序与 JS 版一致）
var All = []*Source{
	{Name: "guduo", Title: "骨朵热度", Fetch: guduoFetch},
	{Name: "douban", Title: "豆瓣热榜", Fetch: doubanFetch},
	{Name: "mgtv", Title: "芒果TV", Fetch: mgtvFetch},
	{Name: "theater", Title: "剧场平台", Fetch: theaterFetch},
	{Name: "bangumi", Title: "番剧", Fetch: bangumiFetch},
}

// ByName 按名字取源（不存在返回 nil）
func ByName(name string) *Source {
	for _, s := range All {
		if s.Name == name {
			return s
		}
	}
	return nil
}

// Has 是否存在该源
func Has(name string) bool { return ByName(name) != nil }

// ===== HTML 解析辅助（与 JS 各源内联的 strip 逻辑一致，不解码实体）=====

var (
	reHTMLTag = regexp.MustCompile(`<[^>]+>`)
	reWS      = regexp.MustCompile(`\s+`)
)

// stripTagsCollapse 去标签 + 压缩空白 + trim（对应 .replace(/<[^>]+>/g,"").replace(/\s+/g," ").trim()）
func stripTagsCollapse(s string) string {
	return strings.TrimSpace(reWS.ReplaceAllString(reHTMLTag.ReplaceAllString(s, ""), " "))
}

// stripTagsTrim 去标签 + trim（对应 .replace(/<[^>]+>/g,"").trim()）
func stripTagsTrim(s string) string {
	return strings.TrimSpace(reHTMLTag.ReplaceAllString(s, ""))
}

// stripTagsSpace 去标签替换为空格（对应 .replace(/<[^>]+>/g," ")，不压缩不 trim）
func stripTagsSpace(s string) string {
	return reHTMLTag.ReplaceAllString(s, " ")
}

// matchTVResult 手动匹配循环（对应 douban/mgtv/theater 三处几乎相同的 for 循环）
// requireAired 为 true 时拦截未开播（theater 版行为）
func matchTVResult(results []tmdb.Result, target, year string, requireAired bool) *tmdb.Result {
	t := strings.ToLower(target)
	today := util.TodayString()
	for i := range results {
		res := &results[i]
		n := strings.ToLower(res.Name)
		o := strings.ToLower(res.OriginalName)
		if !(strings.Contains(n, t) || strings.Contains(o, t) || strings.Contains(t, n)) {
			continue
		}
		fa := res.FirstAirDate
		if year != "" && fa != "" && !strings.HasPrefix(fa, year) {
			continue
		}
		if res.PosterPath == nil || res.BackdropPath == nil {
			continue
		}
		if requireAired && (fa == "" || fa > today) {
			continue
		}
		return res
	}
	return nil
}

// withLastAirDate 构建 Info 并补 lastUpdateDate（fetchLastAirDate 失败时保持原值）
func withLastAirDate(res *tmdb.Result, apiKey string) tmdb.Info {
	info := tmdb.BuildInfo(*res, "tv")
	if lu := tmdb.FetchLastAirDate(res.ID, apiKey); lu != "" {
		info.LastUpdateDate = lu
	}
	return info
}
