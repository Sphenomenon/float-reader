package main

import (
	w "floatreader/internal/win32"
	"fmt"
	"unsafe"
)

type hitRect struct{ x, y, width, height int }

func (r hitRect) contains(x, y int) bool {
	return x >= r.x && x <= r.x+r.width && y >= r.y && y <= r.y+r.height
}
func (a *App) importButton() hitRect {
	r := w.Client(a.hwnd)
	return hitRect{(r.Width() - a.px(176)) / 2, r.Height()/2 + a.px(18), a.px(176), a.px(44)}
}
func (a *App) label(dc uintptr, s string, font, color uintptr, x, y, width, height int, flags uint32) {
	w.G("SelectObject", dc, font)
	w.G("SetTextColor", dc, color)
	w.DrawText(dc, s, w.Rect{Left: int32(x), Top: int32(y), Right: int32(x + width), Bottom: int32(y + height)}, flags)
}
func (a *App) roundRect(dc uintptr, r w.Rect, color uintptr, radius int) {
	b := w.G("CreateSolidBrush", color)
	oldB := w.G("SelectObject", dc, b)
	oldP := w.G("SelectObject", dc, w.G("GetStockObject", 8))
	w.G("RoundRect", dc, w.Signed(int(r.Left)), w.Signed(int(r.Top)), w.Signed(int(r.Right)), w.Signed(int(r.Bottom)), uintptr(radius), uintptr(radius))
	w.G("SelectObject", dc, oldP)
	w.G("SelectObject", dc, oldB)
	w.G("DeleteObject", b)
}
func (a *App) paint(hwnd uintptr) {
	var ps w.Paint
	screen := w.U("BeginPaint", hwnd, uintptr(unsafe.Pointer(&ps)))
	r := w.Client(hwnd)
	if r.Width() < 1 || r.Height() < 1 {
		w.U("EndPaint", hwnd, uintptr(unsafe.Pointer(&ps)))
		return
	}
	dc := w.G("CreateCompatibleDC", screen)
	bmp := w.G("CreateCompatibleBitmap", screen, uintptr(r.Width()), uintptr(r.Height()))
	old := w.G("SelectObject", dc, bmp)
	p := a.pal()
	w.Fill(dc, r, p.BG)
	w.G("SetBkMode", dc, 1)
	pad := a.px(28)
	width := r.Width()
	height := r.Height()
	// A quiet masthead doubles as the drag handle.
	a.roundRect(dc, w.Rect{Left: int32(pad), Top: int32(a.px(22)), Right: int32(pad + a.px(6)), Bottom: int32(a.px(40))}, p.Accent, a.px(4))
	a.label(dc, "隅读", a.uiFont, p.Accent, pad+a.px(16), a.px(18), a.px(56), a.px(27), w.DT_SINGLELINE|w.DT_VCENTER)
	a.label(dc, "始终置顶", a.smallFont, p.Muted, pad+a.px(78), a.px(19), a.px(80), a.px(26), w.DT_SINGLELINE|w.DT_VCENTER)
	a.roundRect(dc, w.Rect{Left: int32(width - a.px(77)), Top: int32(a.px(19)), Right: int32(width - a.px(20)), Bottom: int32(a.px(47))}, p.Soft, a.px(10))
	a.label(dc, "设置", a.smallFont, p.Accent, width-a.px(76), a.px(19), a.px(56), a.px(28), w.DT_CENTER|w.DT_SINGLELINE|w.DT_VCENTER)
	subtitle := "拖动上方移动 · 拖动边缘调整大小"
	if a.bookName != "" {
		subtitle = a.bookName
	}
	a.label(dc, subtitle, a.smallFont, p.Muted, pad, a.px(51), width-pad*2, a.px(20), w.DT_SINGLELINE|w.DT_END_ELLIPSIS)
	w.Fill(dc, w.Rect{Left: int32(pad), Top: int32(a.px(77)), Right: int32(width - pad), Bottom: int32(a.px(78))}, p.Line)
	if len(a.text) == 0 {
		cy := height / 2
		if height >= a.px(360) {
			a.roundRect(dc, w.Rect{Left: int32(width/2 - a.px(25)), Top: int32(cy - a.px(135)), Right: int32(width/2 + a.px(25)), Bottom: int32(cy - a.px(83))}, p.Soft, a.px(14))
			a.label(dc, "读", a.titleFont, p.Accent, width/2-a.px(25), cy-a.px(134), a.px(50), a.px(50), w.DT_CENTER|w.DT_VCENTER|w.DT_SINGLELINE)
		}
		titleY := max(a.px(87), cy-a.px(58))
		a.label(dc, "把故事，留在手边。", a.titleFont, p.Ink, pad, titleY, width-2*pad, a.px(36), w.DT_CENTER|w.DT_SINGLELINE)
		if height >= a.px(300) {
			a.label(dc, "导入 TXT，开始一段安静的阅读", a.smallFont, p.Muted, pad, cy-a.px(17), width-2*pad, a.px(22), w.DT_CENTER|w.DT_SINGLELINE)
		}
		b := a.importButton()
		a.roundRect(dc, w.Rect{Left: int32(b.x), Top: int32(b.y), Right: int32(b.x + b.width), Bottom: int32(b.y + b.height)}, p.Accent, a.px(12))
		a.label(dc, "＋  导入 TXT", a.uiFont, p.BG, b.x, b.y, b.width, b.height, w.DT_CENTER|w.DT_SINGLELINE|w.DT_VCENTER)
		if height >= a.px(350) {
			a.label(dc, "也可以把文件拖到这里", a.smallFont, p.Muted, pad, b.y+b.height+a.px(14), width-2*pad, a.px(22), w.DT_CENTER|w.DT_SINGLELINE)
		}
		a.label(dc, "隐藏 / 显示  "+keyName(a.cfg.Keys[actBoss]), a.smallFont, p.Muted, pad, height-a.px(39), width-2*pad, a.px(22), w.DT_CENTER|w.DT_SINGLELINE)
	} else if len(a.pages) > 0 {
		pge := a.pages[clamp(a.page, 0, len(a.pages)-1)]
		content := a.contentRect()
		w.G("SaveDC", dc)
		w.G("IntersectClipRect", dc, w.Signed(int(content.Left)), w.Signed(int(content.Top)), w.Signed(int(content.Right)), w.Signed(int(content.Bottom)))
		for i, l := range pge.Lines {
			a.label(dc, string(a.text[l.Start:l.End]), a.bodyFont, p.Ink, int(content.Left), int(content.Top)+i*a.lineHeight(), content.Width(), a.lineHeight(), w.DT_SINGLELINE)
		}
		w.G("RestoreDC", dc, w.Signed(-1))
		y := height - a.px(52)
		a.label(dc, "‹", a.titleFont, p.Accent, a.px(20), y, a.px(34), a.px(32), w.DT_CENTER|w.DT_SINGLELINE|w.DT_VCENTER)
		a.label(dc, "›", a.titleFont, p.Accent, width-a.px(54), y, a.px(34), a.px(32), w.DT_CENTER|w.DT_SINGLELINE|w.DT_VCENTER)
		s := fmt.Sprintf("%d / %d 页  ·  本页 %d 字", a.page+1, len(a.pages), pge.End-pge.Start)
		if a.toast != "" {
			s = a.toast
		}
		a.label(dc, s, a.smallFont, p.Muted, a.px(60), y, width-a.px(120), a.px(30), w.DT_CENTER|w.DT_SINGLELINE|w.DT_VCENTER|w.DT_END_ELLIPSIS)
		bar := w.Rect{Left: int32(pad), Top: int32(height - a.px(13)), Right: int32(width - pad), Bottom: int32(height - a.px(11))}
		w.Fill(dc, bar, p.Line)
		bar.Right = bar.Left + int32(float64(bar.Width())*float64(pge.End)/float64(max(1, len(a.text))))
		w.Fill(dc, bar, p.Accent)
	}
	// Small resize grip in the lower-right corner.
	for i := 0; i < 3; i++ {
		x := width - a.px(6+i*4)
		y := height - a.px(6)
		w.Fill(dc, w.Rect{Left: int32(x), Top: int32(y - a.px(2)), Right: int32(x + a.px(2)), Bottom: int32(y)}, p.Muted)
	}
	w.G("BitBlt", screen, 0, 0, uintptr(width), uintptr(height), dc, 0, 0, 0x00cc0020)
	w.G("SelectObject", dc, old)
	w.G("DeleteObject", bmp)
	w.G("DeleteDC", dc)
	w.U("EndPaint", hwnd, uintptr(unsafe.Pointer(&ps)))
}
