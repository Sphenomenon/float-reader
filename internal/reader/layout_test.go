package reader

import (
	"strings"
	"testing"
)

func mono(rs []rune, width int) int { return min(len(rs), width) }

func TestPaginationPreservesAllText(t *testing.T) {
	for _, s := range []string{"", "\n\n\n", "第一章\n山路很长。\n\n第二章\n雨停了🌧️", strings.Repeat("很长的段落", 10000), "a long Latin word and abcdefghijklmnopqrstuvwxyz"} {
		for _, limit := range []int{0, 1, 7, 50} {
			text := Normalize(s)
			pages := Paginate(text, 8, 3, limit, mono)
			pos := 0
			for _, p := range pages {
				if p.Start != pos || p.End < p.Start {
					t.Fatalf("gap: %+v after %d", p, pos)
				}
				if p.End == p.Start && len(text) > 0 {
					t.Fatal("empty nonterminal page")
				}
				if len(p.Lines) > 3 {
					t.Fatal("page overflows height")
				}
				if limit > 0 && p.End-p.Start > limit {
					t.Fatal("page exceeds cap")
				}
				for _, l := range p.Lines {
					if l.End-l.Start > 8 || l.Start < p.Start || l.End > p.End {
						t.Fatal("invalid line", l)
					}
				}
				pos = p.End
			}
			if pos != len(text) {
				t.Fatalf("text lost: %d != %d", pos, len(text))
			}
		}
	}
}

func TestResizeKeepsReadingAnchor(t *testing.T) {
	rs := Normalize(strings.Repeat("用一段中文检查分页。", 100))
	old := Paginate(rs, 12, 5, 0, mono)
	anchor := old[7].Start
	for _, w := range []int{1, 7, 20, 100} {
		p := Paginate(rs, w, 3, 0, mono)
		i := PageAt(p, anchor)
		if p[i].Start > anchor || p[i].End <= anchor {
			t.Fatal("anchor lost")
		}
	}
}

func TestNormalization(t *testing.T) {
	got := string(Normalize("\ufeff甲\r\n乙\r丙\x00\t丁"))
	if got != "甲\n乙\n丙    丁" {
		t.Fatal(got)
	}
}
