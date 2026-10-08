package util

import (
	"encoding/json"
	"testing"
)

// 期望值来源：Node 原版 lib/util.js 对同一组输入的实际输出
func TestCleanFuncs(t *testing.T) {
	inputs := []string{
		"庆余年 第二季 (2024)",
		"进击的巨人 最终季 Part.2",
		"狐妖小红娘·月红篇（2024）",
		"歌手2024",
		"名侦探柯南 2024",
		" 多余 空白 ",
		"斗罗大陆 第1季",
		"三体 Season 2",
		"开始推理吧（2024） 8",
		"某某年番特别篇 3",
	}
	cases := []struct {
		name string
		fn   func(string) string
		want []string
	}{
		{"CleanSeason", CleanSeason, []string{
			"庆余年 (2024)", "进击的巨人 最终季 Part.2", "狐妖小红娘·月红篇（2024）",
			"歌手2024", "名侦探柯南 2024", "多余 空白",
			"斗罗大陆", "三体", "开始推理吧（2024） 8", "某某年番特别篇 3",
		}},
		{"CleanBrackets", CleanBrackets, []string{
			"庆余年 第二季", "进击的巨人 最终季 Part.2", "狐妖小红娘·月红篇",
			"歌手2024", "名侦探柯南 2024", "多余 空白",
			"斗罗大陆 第1季", "三体 Season 2", "开始推理吧 8", "某某年番特别篇 3",
		}},
		{"CleanGuduo", CleanGuduo, []string{
			"庆余年", "进击的巨人 最终季 Part.", "狐妖小红娘·月红篇",
			"歌手", "名侦探柯南", "多余 空白",
			"斗罗大陆", "三体", "开始推理吧", "某某",
		}},
		{"CleanDouban", CleanDouban, []string{
			"庆余年", "进击的巨人 最终季 Part.2", "狐妖小红娘·月红篇",
			"歌手2024", "名侦探柯南 2024", "多余 空白",
			"斗罗大陆", "三体", "开始推理吧（2024） 8", "某某年番特别篇 3",
		}},
		{"CleanMgtv", CleanMgtv, []string{
			"庆余年", "进击的巨人 最终季 Part.2", "狐妖小红娘·月红篇",
			"歌手2024", "名侦探柯南 2024", "多余 空白",
			"斗罗大陆", "三体", "开始推理吧 8", "某某年番特别篇 3",
		}},
		{"CleanAnime", CleanAnime, []string{
			"庆余年 (2024)", "进击的巨人 最终季 Part.2", "狐妖小红娘·月红篇（2024）",
			"歌手2024", "名侦探柯南", "多余 空白",
			"斗罗大陆", "三体", "开始推理吧（2024） 8", "某某年番特别篇 3",
		}},
	}
	for _, c := range cases {
		for i, in := range inputs {
			if got := c.fn(in); got != c.want[i] {
				t.Errorf("%s(%q) = %q, want %q", c.name, in, got, c.want[i])
			}
		}
	}
}

func TestOMapOrder(t *testing.T) {
	m := NewOMap().Set("b", 1).Set("a", 2).Set("b", 3)
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"b":3,"a":2}` {
		t.Errorf("got %s, want {\"b\":3,\"a\":2}", b)
	}
}
