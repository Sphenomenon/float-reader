// Package reader contains the platform-independent pagination engine.
package reader

import (
	"strings"
	"unicode"
)

type Line struct {
	Start int
	End   int
}

type Page struct {
	Start int
	End   int
	Lines []Line
}

// Fit returns the number of runes that fit within a line's pixel width.
type Fit func([]rune, int) int

func Normalize(s string) []rune {
	s = strings.TrimPrefix(s, "\ufeff")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "\x00", "")
	s = strings.ReplaceAll(s, "\t", "    ")
	return []rune(s)
}

// Paginate consumes every rune exactly once. Limit caps the page's character
// count as well as its width and number of lines.
func Paginate(text []rune, width, rows, limit int, fit Fit) []Page {
	return paginate(text, width, rows, limit, fit, false)
}

// PaginatePage lays out only the first page. Full reports whether the page
// stopped because its rows or character limit were filled. Callers can use it
// with a small text window and load more text only when full is false.
func PaginatePage(text []rune, width, rows, limit int, fit Fit) (page Page, full bool) {
	pages := paginate(text, width, rows, limit, fit, true)
	page = pages[0]
	full = page.End < len(text) || len(page.Lines) >= max(1, rows) || limit > 0 && page.End-page.Start >= limit
	return page, full
}

func paginate(text []rune, width, rows, limit int, fit Fit, firstOnly bool) []Page {
	if width < 1 {
		width = 1
	}
	if rows < 1 {
		rows = 1
	}
	pages := make([]Page, 0, len(text)/300+1)
	for pos := 0; pos < len(text); {
		p := Page{Start: pos}
		for row := 0; row < rows && pos < len(text); row++ {
			if limit > 0 && pos-p.Start >= limit {
				break
			}
			end := pos
			// Bound candidate size so huge unbroken paragraphs stay linear.
			max := min(len(text), pos+min(4096, max(64, width*2)))
			if limit > 0 {
				max = min(max, p.Start+limit)
			}
			for end < max && text[end] != '\n' {
				end++
			}
			if end == pos && text[pos] == '\n' {
				p.Lines = append(p.Lines, Line{pos, pos})
				pos++
				continue
			}
			n := fit(text[pos:end], width)
			if n < 1 {
				n = 1
			}
			if n > end-pos {
				n = end - pos
			}
			// Prefer whole Latin words; Chinese text wraps at character boundaries.
			if pos+n < end && n > 2 && asciiWord(text[pos+n-1]) && asciiWord(text[pos+n]) {
				for j := n - 1; j >= n/2; j-- {
					if unicode.IsSpace(text[pos+j]) {
						n = j + 1
						break
					}
				}
			}
			if pos+n < end && n > 1 {
				for n > 1 && strings.ContainsRune("，。！？；：、）》】」』”’…,.!?;:)]}", text[pos+n]) {
					n--
				}
				if n > 1 && strings.ContainsRune("（《【「『“‘([{", text[pos+n-1]) {
					n--
				}
			}
			p.Lines = append(p.Lines, Line{pos, pos + n})
			pos += n
			if pos == end && pos < len(text) && text[pos] == '\n' && (limit <= 0 || pos-p.Start < limit) {
				pos++
			}
		}
		p.End = pos
		pages = append(pages, p)
		if firstOnly {
			break
		}
	}
	if len(pages) == 0 {
		pages = append(pages, Page{})
	}
	return pages
}

func asciiWord(r rune) bool { return r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)) }

func PageAt(pages []Page, offset int) int {
	lo, hi := 0, len(pages)
	for lo < hi {
		mid := lo + (hi-lo)/2
		if pages[mid].End <= offset {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo >= len(pages) {
		return max(0, len(pages)-1)
	}
	return lo
}

func Capacity(width, height, fontSize, lineHeight int) int {
	return max(1, width/max(1, fontSize)) * max(1, height/max(1, lineHeight))
}
