// Package widget — 生成聚合 fw/rex 模块（5 源榜单，对应原 lib/widget.js）
package widget

import (
	"fmt"
	"strings"
)

type opt struct{ title, value string }

var doubanChannels = []opt{
	{"全部劇集", "tv"}, {"大陸劇集", "tv_domestic"}, {"歐美劇集", "tv_american"},
	{"日本劇集", "tv_japanese"}, {"南韓劇集", "tv_korean"}, {"動漫番劇", "tv_animation"},
	{"紀錄片", "tv_documentary"}, {"大陸綜藝", "show_domestic"}, {"國外綜藝", "show_foreign"},
}

var theaterBrands = []opt{
	{"迷霧劇場", "迷雾剧场"}, {"白夜劇場", "白夜剧场"}, {" X 劇場", "X剧场"},
	{"瑪卡的片單", "玛卡巴卡的悬疑剧"}, {"橫屏短劇", "横屏短剧"}, {"生花劇場", "生花剧场"},
	{"大家劇場", "大家剧场"}, {"小逗劇場", "小逗剧场"}, {"十分劇場", "十分剧场"},
	{"板凳單元", "板凳单元"}, {"螢火單元", "萤火单元"}, {"正午陽光", "正午阳光"},
	{"戀戀劇場", "恋恋剧场"}, {"懸疑劇場", "悬疑剧场"}, {"微塵劇場", "微尘剧场"},
}

var sortOptions = []opt{
	{"默認原序", "default"},
	{"最近更新", "latestUpdate"},
	{"最近發佈", "latestRelease"},
	{"熱度最高", "hottest"},
	{"流行趨勢", "trend"},
	{"高分優先", "highestRating"},
}

var guduoCategories = []opt{{"劇集", "剧集"}, {"綜藝", "综艺"}, {"動漫", "动漫"}, {"電影", "电影"}}
var mgtvTypes = []opt{{"全部劇集", "tv"}, {"王牌綜藝", "show"}}
var theaterStatuses = []opt{{"全部", "all"}, {"已開播", "aired"}, {"即將推出", "upcoming"}}

func enumParam(name, title, value string, options []opt) string {
	parts := make([]string, len(options))
	for i, o := range options {
		parts[i] = fmt.Sprintf(`{ title: "%s", value: "%s" }`, o.title, o.value)
	}
	return fmt.Sprintf(`      { name: "%s", title: "%s", type: "enumeration", value: "%s", enumOptions: [%s] }`,
		name, title, value, strings.Join(parts, ","))
}

func pageParam() string { return `      { name: "page", title: "页码", type: "page", startPage: 1 }` }

func sortParam() string { return enumParam("sort", "排序方式", "default", sortOptions) }

const widgetFuncs = `function toVideo(it) {
  return {
    id: Number(it.tmdbId || it.id) || 0,
    type: "tmdb",
    mediaType: it.mediaType || (it.type === "movie" ? "movie" : "tv"),
    title: it.title || it.tmdbTitle || "",
    posterPath: it.posterPath,
    backdropPath: it.backdropPath,
    rating: it.rating,
    releaseDate: it.releaseDate,
    description: it.description || it.overview || "",
  };
}

// 排序：默认原序 / 最近更新(lastUpdateDate) / 最近发布(releaseDate) / 热度(popularity) / 流行(popularity) / 高分(rating)
function sortItems(list, sort) {
  const s = sort || "default";
  const arr = (list || []).slice();
  if (s === "default") return arr;
  const key = s === "latestUpdate" ? "lastUpdateDate"
    : s === "latestRelease" ? "releaseDate"
    : (s === "hottest" || s === "trend") ? "popularity" : "rating";
  if (key === "lastUpdateDate" || key === "releaseDate") {
    arr.sort((a, b) => String(b[key] || "").localeCompare(String(a[key] || "")));
  } else {
    arr.sort((a, b) => (Number(b[key]) || 0) - (Number(a[key]) || 0));
  }
  return arr;
}

function paginate(list, page, size = 24) {
  const p = Number(page || 1);
  const start = (p - 1) * size;
  return (list || []).slice(start, start + size).map(toVideo);
}

async function loadGuduo(params = {}) {
  const res = await Widget.http.get((params.baseUrl || BASE) + "/data/guduo.json");
  const list = (res.data && res.data.categories && res.data.categories[params.category || "剧集"]) || [];
  return paginate(sortItems(list, params.sort), params.page);
}

async function loadDouban(params = {}) {
  const res = await Widget.http.get((params.baseUrl || BASE) + "/data/douban.json");
  const list = (res.data && res.data[params.channel || "tv"]) || [];
  return paginate(sortItems(list, params.sort), params.page);
}

async function loadMangoTV(params = {}) {
  const res = await Widget.http.get((params.baseUrl || BASE) + "/data/mgtv.json");
  const list = (res.data && res.data[params.sort_by || "tv"]) || [];
  return paginate(sortItems(list, params.sort), params.page);
}

async function loadTheater(params = {}) {
  const res = await Widget.http.get((params.baseUrl || BASE) + "/data/theater.json");
  const brand = (res.data || {})[params.brand || "迷雾剧场"];
  if (!brand) return [];
  let list = [];
  if (params.status === "aired") list = brand.aired || [];
  else if (params.status === "upcoming") list = brand.upcoming || [];
  else list = [...(brand.upcoming || []), ...(brand.aired || [])];
  return paginate(sortItems(list, params.sort), params.page);
}

async function loadBangumi(params = {}) {
  const res = await Widget.http.get((params.baseUrl || BASE) + "/data/bangumi.json");
  const list = (res.data && res.data.hot_anime) || [];
  return paginate(sortItems(list, params.sort), params.page);
}
`

// Generate 生成聚合模块 JS 代码（内容与原 JS 版逐字节一致）
func Generate(baseURL string) string {
	url := strings.TrimRight(baseURL, "/")
	if url == "" {
		url = "http://127.0.0.1:5555"
	}

	head := `/**
 * 全站榜单聚合 · 自动生成 (VPS 面板)
 * 数据源基础地址: ` + url + `
 * 包含：骨朵热度 / 豆瓣热榜 / 芒果TV / 剧场平台 / 番剧榜
 */
WidgetMetadata = {
  id: "makka.vps.aggregator",
  title: "全站榜单聚合",
  description: "聚合榜单（VPS 每日抓取）",
  author: "𝙈𝙖𝙠𝙠𝙖𝙋𝙖𝙠𝙠𝙖",
  site: "https://t.me/MakkaPakkaOvO",
  version: "1.0.0",
  requiredVersion: "0.0.1",
  globalParams: [
    { name: "baseUrl", title: "VPS 数据地址", type: "input", placeholders: [{ title: "VPS 数据接口", value: "` + url + `" }] },
  ],
  modules: [
`

	modGuduo := `    {
      id: "loadGuduo",
      title: "骨朵热度",
      functionName: "loadGuduo",
      cacheDuration: 3600,
      params: [
` + enumParam("category", "分类", "剧集", guduoCategories) + ",\n" +
		sortParam() + ",\n" +
		pageParam() + ",\n" +
		`      ],
    },
`

	modDouban := `    {
      id: "loadDouban",
      title: "豆瓣熱榜",
      functionName: "loadDouban",
      cacheDuration: 3600,
      params: [
` + enumParam("channel", "榜單分類", "tv", doubanChannels) + ",\n" +
		sortParam() + ",\n" +
		pageParam() + ",\n" +
		`      ],
    },
`

	modMgtv := `    {
      id: "loadMangoTV",
      title: "芒果TV熱榜",
      functionName: "loadMangoTV",
      cacheDuration: 3600,
      params: [
` + enumParam("sort_by", "類型", "tv", mgtvTypes) + ",\n" +
		sortParam() + ",\n" +
		pageParam() + ",\n" +
		`      ],
    },
`

	modTheater := `    {
      id: "loadTheater",
      title: "各平臺劇場",
      functionName: "loadTheater",
      cacheDuration: 3600,
      params: [
` + enumParam("brand", "劇場品牌", "迷雾剧场", theaterBrands) + ",\n" +
		enumParam("status", "播出狀態", "all", theaterStatuses) + ",\n" +
		sortParam() + ",\n" +
		pageParam() + ",\n" +
		`      ],
    },
`

	modBangumi := `    {
      id: "loadBangumi",
      title: "熱門番劇",
      functionName: "loadBangumi",
      cacheDuration: 3600,
      params: [
` + sortParam() + ",\n" +
		pageParam() + ",\n" +
		`      ],
    },
`

	tail := `  ],
};

const BASE = "` + url + `";

` + widgetFuncs

	return head + modGuduo + modDouban + modMgtv + modTheater + modBangumi + tail
}
