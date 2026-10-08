// Package util — 通用工具：清洗/HTML 解析/时间/JSON 存取
// （对应原 lib/util.js，行为保持一致）
package util

import (
	"bytes"
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"time"
)

var (
	reTag        = regexp.MustCompile(`<[^>]+>`)
	reWhitespace = regexp.MustCompile(`\s+`)
	reSeason     = regexp.MustCompile(`第[一二三四五六七八九十百\d]+[季期部章]`)
	reSeasonEN   = regexp.MustCompile(`(?i)Season\s*\d+`)
	reBrackets   = regexp.MustCompile(`\(.*?\)|（.*?）|\[.*?\]|【.*?】`)
	reTrailNum   = regexp.MustCompile(`\s*\d+$`)
	reTrailYear  = regexp.MustCompile(`[（(]\s*(\d{4})\s*[)）]$`)
	reAnimeYear  = regexp.MustCompile(` \d{4}$`)
)

// DecodeEntities 按原 JS 的链式替换顺序（先 &amp; 再其余）保持一致
func DecodeEntities(s string) string {
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	return s
}

// StripTags 去标签 + 解码实体 + 压缩空白
func StripTags(s string) string {
	noTags := reTag.ReplaceAllString(s, " ")
	return strings.TrimSpace(reWhitespace.ReplaceAllString(DecodeEntities(noTags), " "))
}

// CleanSeason 去季/期/部/章 与 Season N
func CleanSeason(t string) string {
	t = reSeason.ReplaceAllString(t, "")
	t = reSeasonEN.ReplaceAllString(t, "")
	return strings.TrimSpace(reWhitespace.ReplaceAllString(t, " "))
}

// CleanBrackets 去括号内容
func CleanBrackets(t string) string {
	t = reBrackets.ReplaceAllString(t, "")
	return strings.TrimSpace(reWhitespace.ReplaceAllString(t, " "))
}

// CleanGuduo 骨朵：去季/期/部 + 括号 + 尾部数字
func CleanGuduo(t string) string {
	t = CleanBrackets(CleanSeason(t))
	t = strings.ReplaceAll(t, "年番", "")
	t = strings.ReplaceAll(t, "特别篇", "")
	t = reTrailNum.ReplaceAllString(t, "")
	return strings.TrimSpace(reWhitespace.ReplaceAllString(t, " "))
}

// CleanDouban 豆瓣/剧场：去尾部年份括号 + 去季 + Season
func CleanDouban(t string) string {
	t = reTrailYear.ReplaceAllString(t, "")
	t = reSeason.ReplaceAllString(t, "")
	t = reSeasonEN.ReplaceAllString(t, "")
	return strings.TrimSpace(reWhitespace.ReplaceAllString(t, " "))
}

// CleanMgtv 芒果：去季/期/部/章 + 括号
func CleanMgtv(t string) string {
	return CleanBrackets(CleanSeason(t))
}

// CleanAnime 番剧：去季/期/部/章 + Season + 尾部年份（含前导空格）
func CleanAnime(t string) string {
	t = reSeason.ReplaceAllString(t, "")
	t = reSeasonEN.ReplaceAllString(t, "")
	t = reAnimeYear.ReplaceAllString(t, "")
	return strings.TrimSpace(reWhitespace.ReplaceAllString(t, " "))
}

// ===== 北京时间（与 JS 版 Intl Asia/Shanghai 行为一致）=====

var bjLoc = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}()

// BjNow 当前北京时间
func BjNow() time.Time { return time.Now().In(bjLoc) }

// BjYesterday 昨天日期 YYYY-MM-DD（北京时区）
func BjYesterday() string { return BjNow().AddDate(0, 0, -1).Format("2006-01-02") }

// TodayString 今天日期 YYYY-MM-DD（北京时区）
func TodayString() string { return BjNow().Format("2006-01-02") }

// BjStamp 时间戳 YYYY/MM/DD HH:mm:ss（北京时区）
func BjStamp() string { return BjNow().Format("2006/01/02 15:04:05") }

// ===== JSON 存取 =====

// LoadJSON 读取 JSON 到 v，成功返回 true（解析失败返回 false，不报错）
func LoadJSON(file string, v any) bool {
	b, err := os.ReadFile(file)
	if err != nil {
		return false
	}
	return json.Unmarshal(b, v) == nil
}

// SaveJSON 2 空格缩进写入（不转义 HTML、不含尾部换行，对齐 JS 版 JSON.stringify 输出）
func SaveJSON(file string, v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return err
	}
	return os.WriteFile(file, bytes.TrimRight(buf.Bytes(), "\n"), 0o644)
}

// ===== 有序 JSON 对象（保持插入顺序，对齐 JS 对象序列化行为）=====

// OMap 按插入顺序序列化的 JSON 对象
type OMap struct {
	keys []string
	m    map[string]any
}

// NewOMap 新建空有序对象
func NewOMap() *OMap { return &OMap{m: map[string]any{}} }

// Set 设置键值（新键追加到末尾）
func (o *OMap) Set(k string, v any) *OMap {
	if _, ok := o.m[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.m[k] = v
	return o
}

// Get 按键取值
func (o *OMap) Get(k string) (any, bool) {
	v, ok := o.m[k]
	return v, ok
}

// MarshalJSON 按插入顺序输出
func (o *OMap) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		vb, err := json.Marshal(o.m[k])
		if err != nil {
			return nil, err
		}
		buf.Write(kb)
		buf.WriteByte(':')
		buf.Write(vb)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}
