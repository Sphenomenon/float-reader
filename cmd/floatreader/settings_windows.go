package main

import (
	w "floatreader/internal/win32"
	"fmt"
	"strconv"
	"unsafe"
)

const (
	fWidth = 201 + iota
	fHeight
	fFont
	fSpacing
	fChars
	fOpacity
	fTextOpacity
	fTheme
	fGlobal
	fHelp
	fSave
	fCancel
	fReset
	fStatus
)

func (a *App) spx(n int) int { return n * max(72, a.settingsDPI) / 96 }
func (a *App) control(class, text string, style uintptr, x, y, width, height, id int) uintptr {
	h := w.U("CreateWindowExW", 0, uintptr(unsafe.Pointer(w.Str(class))), uintptr(unsafe.Pointer(w.Str(text))), w.WS_CHILD|w.WS_VISIBLE|style, uintptr(a.spx(x)), uintptr(a.spx(y)), uintptr(a.spx(width)), uintptr(a.spx(height)), a.dialog, uintptr(id), a.instance, 0)
	w.U("SendMessageW", h, 0x30, a.settingsFont, 1)
	if id != 0 {
		a.fields[id] = h
	}
	return h
}
func (a *App) edit(label, value string, x, y, width, id int) {
	a.control("STATIC", label, 0, x, y, width, 22, 0)
	a.control("EDIT", value, w.WS_TABSTOP|w.WS_BORDER|0x80|0x2000, x, y+25, width, 30, id)
	w.U("SendMessageW", a.fields[id], 0xc5, 5, 0) // EM_SETLIMITTEXT
}
func (a *App) openSettings() {
	if a.dialog != 0 {
		w.U("SetForegroundWindow", a.dialog)
		return
	}
	if a.modal {
		return
	}
	a.save()
	a.modal = true
	a.syncTextLayer()
	a.fields = map[int]uintptr{}
	a.draftKeys = a.cfg.Keys
	a.capturing = -1
	a.charEdited = false
	a.settingsCapacity = a.capacity()
	if a.cfg.CharLimit > 0 {
		a.settingsCapacity = a.cfg.CharLimit
	}
	r := w.Bounds(a.hwnd)
	work := w.WorkArea(a.hwnd)
	a.settingsDPI = min(a.dpi, min((work.Height()-16)*96/756, (work.Width()-16)*96/594))
	face := a.fontFace
	if face == "" {
		face = "Microsoft YaHei UI"
	}
	a.settingsFont = w.G("CreateFontW", w.Signed(-a.spx(14)), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(w.Str(face))))
	width, height := a.spx(594), a.spx(756)
	// Keep the entire dialog inside the monitor's work area.
	x := clamp(int(r.Left)+(r.Width()-width)/2, int(work.Left), max(int(work.Left), int(work.Right)-width))
	y := clamp(int(r.Top)+(r.Height()-height)/2, int(work.Top), max(int(work.Top), int(work.Bottom)-height))
	a.dialog = w.U("CreateWindowExW", w.WS_EX_TOPMOST|w.WS_EX_TOOLWINDOW|w.WS_EX_CONTROLPARENT, uintptr(unsafe.Pointer(w.Str(settingsClass))), uintptr(unsafe.Pointer(w.Str("阅读设置 · 隅读"))), w.WS_POPUP|w.WS_BORDER, w.Signed(x), w.Signed(y), uintptr(width), uintptr(height), a.hwnd, 0, a.instance, 0)
	if a.dialog == 0 {
		a.modal = false
		a.syncTextLayer()
		w.Message(a.hwnd, "无法打开设置。", appTitle, 0x10)
		return
	}
	a.control("STATIC", "阅读设置", 0, 26, 21, 240, 32, 0)
	a.control("STATIC", "修改后点击“保存设置”。", 0, 26, 57, 540, 22, 0)
	a.edit("窗口宽度", strconv.Itoa(a.cfg.Width), 26, 99, 122, fWidth)
	a.edit("窗口高度", strconv.Itoa(a.cfg.Height), 164, 99, 122, fHeight)
	a.edit("字号", strconv.Itoa(a.cfg.FontSize), 302, 99, 122, fFont)
	a.edit("行距 %", strconv.Itoa(a.cfg.LineSpace), 440, 99, 122, fSpacing)
	a.edit("每页目标字数", strconv.Itoa(a.settingsCapacity), 26, 170, 164, fChars)
	a.edit("背景不透明度 %", strconv.Itoa(a.cfg.Opacity), 208, 170, 164, fOpacity)
	a.edit("文字不透明度 %", strconv.Itoa(a.cfg.TextOpacity), 390, 170, 172, fTextOpacity)
	a.control("STATIC", "阅读配色", 0, 26, 242, 172, 22, 0)
	theme := a.control("COMBOBOX", "", w.WS_TABSTOP|3|0x200000, 26, 267, 172, 140, fTheme)
	for _, s := range []string{"暖白", "深色", "浅蓝"} {
		w.U("SendMessageW", theme, 0x143, 0, uintptr(unsafe.Pointer(w.Str(s))))
	}
	w.U("SendMessageW", theme, 0x14e, uintptr(a.cfg.Theme), 0)
	a.control("STATIC", "背景和文字可分别调节。背景设为 0 时只留下正文。", 0, 26, 312, 540, 22, 0)
	a.control("STATIC", "快捷键", 0, 26, 344, 220, 25, 0)
	a.control("STATIC", "点击输入框，直接按下新快捷键。", 0, 26, 372, 540, 22, 0)
	for i, name := range actionNames {
		y := 406 + i*38
		a.control("STATIC", name, 0, 26, y+5, 275, 26, 0)
		h := a.control("EDIT", keyName(a.cfg.Keys[i]), w.WS_TABSTOP|w.WS_BORDER|0x800|0x80, 315, y, 247, 30, 300+i)
		old := w.U("SetWindowLongPtrW", h, w.Signed(-4), keyProcPtr)
		a.editOld[h] = old
	}
	a.control("BUTTON", "在其他软件里也能翻页", w.WS_TABSTOP|3, 26, 604, 536, 28, fGlobal)
	if a.cfg.GlobalNav {
		w.U("SendMessageW", a.fields[fGlobal], 0xf1, 1, 0)
	}
	a.control("STATIC", "老板键随时可用。全局翻页建议用 Ctrl / Alt / Shift 组合。", 0, 26, 636, 540, 23, 0)
	a.control("STATIC", "", 0, 26, 664, 540, 34, fStatus)
	a.control("BUTTON", "恢复默认", w.WS_TABSTOP, 26, 710, 106, 32, fReset)
	a.control("BUTTON", "取消", w.WS_TABSTOP, 330, 710, 106, 32, fCancel)
	a.control("BUTTON", "保存设置", w.WS_TABSTOP|1, 452, 710, 110, 32, fSave)
	w.U("EnableWindow", a.hwnd, 0)
	w.U("ShowWindow", a.dialog, w.SW_SHOW)
	w.U("SetForegroundWindow", a.dialog)
	w.U("SetFocus", a.fields[fWidth])
}

func (a *App) closeSettings() {
	if a.dialog == 0 {
		return
	}
	w.U("EnableWindow", a.hwnd, 1)
	h := a.dialog
	a.dialog = 0
	a.modal = false
	a.capturing = -1
	w.U("DestroyWindow", h)
	w.G("DeleteObject", a.settingsFont)
	a.settingsFont = 0
	a.fields = nil
	a.editOld = map[uintptr]uintptr{}
	a.syncTextLayer()
	if !a.hidden {
		w.U("SetForegroundWindow", a.hwnd)
	}
}
func (a *App) settingsError(s string) { w.SetText(a.fields[fStatus], s); w.U("MessageBeep", 0x30) }
func (a *App) applySettings() bool {
	c := a.cfg
	read := func(id, lo, hi int, name string) (int, error) {
		n, err := strconv.Atoi(w.Text(a.fields[id]))
		if err != nil || n < lo || n > hi {
			return 0, fmt.Errorf("%s应为 %d–%d 的整数", name, lo, hi)
		}
		return n, nil
	}
	for _, v := range []struct {
		id, lo, hi int
		name       string
		out        *int
	}{{fWidth, 320, 2000, "宽度", &c.Width}, {fHeight, 230, 2000, "高度", &c.Height}, {fFont, 14, 40, "字号", &c.FontSize}, {fSpacing, 120, 220, "行距", &c.LineSpace}, {fOpacity, 0, 100, "背景不透明度", &c.Opacity}, {fTextOpacity, 0, 100, "文字不透明度", &c.TextOpacity}} {
		n, err := read(v.id, v.lo, v.hi, v.name)
		if err != nil {
			a.settingsError(err.Error())
			w.U("SetFocus", a.fields[v.id])
			return false
		}
		*v.out = n
	}
	chars, err := read(fChars, 10, 20000, "字数")
	if err != nil {
		a.settingsError(err.Error())
		return false
	}
	c.Theme = int(w.U("SendMessageW", a.fields[fTheme], 0x147, 0, 0))
	c.Theme = clamp(c.Theme, 0, 2)
	c.GlobalNav = w.U("SendMessageW", a.fields[fGlobal], 0xf0, 0, 0) == 1
	c.Keys = a.draftKeys
	if err = validateKeys(c); err != nil {
		a.settingsError(err.Error())
		return false
	}
	c.CharLimit = a.cfg.CharLimit
	if c.Width != a.cfg.Width || c.Height != a.cfg.Height || c.FontSize != a.cfg.FontSize || c.LineSpace != a.cfg.LineSpace {
		c.CharLimit = 0
	}
	if chars != a.settingsCapacity {
		work := w.WorkArea(a.hwnd)
		maxW, maxH := a.dip(work.Width()), a.dip(work.Height())
		cols := max(1, (c.Width-56)/c.FontSize)
		rows := (chars + cols - 1) / cols
		line := c.FontSize * c.LineSpace / 100
		maximum := max(1, (maxW-56)/c.FontSize) * max(1, (maxH-154)/line)
		if chars > maximum {
			a.settingsError(fmt.Sprintf("当前屏幕和字号最多能放下约 %d 字，请减少字数或缩小字号。", maximum))
			return false
		}
		c.Height = max(230, 154+rows*line)
		if c.Height > maxH {
			c.Height = maxH
			rows = max(1, (maxH-154)/line)
			cols = (chars + rows - 1) / rows
			c.Width = clamp(56+cols*c.FontSize, 320, maxW)
		}
		c.CharLimit = chars
	}
	if err = a.registerKeys(c); err != nil {
		restoreErr := a.registerKeys(a.cfg)
		if restoreErr != nil {
			err = fmt.Errorf("%v；原快捷键恢复失败，请通过托盘显示窗口", err)
		}
		a.settingsError(err.Error())
		return false
	}
	a.closeSettings()
	a.cfg = c
	a.rebuildFonts()
	a.applyAppearance()
	w.U("SetWindowPos", a.hwnd, w.Topmost, 0, 0, uintptr(a.px(c.Width)), uintptr(a.px(c.Height)), w.SWP_NOMOVE|w.SWP_NOACTIVATE)
	a.ensureOnScreen()
	a.paginate()
	a.save()
	a.notify("设置已保存")
	return true
}
func (a *App) resetSettings() {
	c := defaults()
	for id, n := range map[int]int{fWidth: c.Width, fHeight: c.Height, fFont: c.FontSize, fSpacing: c.LineSpace, fOpacity: c.Opacity, fTextOpacity: c.TextOpacity, fChars: 228} {
		w.SetText(a.fields[id], strconv.Itoa(n))
	}
	w.U("SendMessageW", a.fields[fTheme], 0x14e, 0, 0)
	w.U("SendMessageW", a.fields[fGlobal], 0xf1, 0, 0)
	a.draftKeys = c.Keys
	for i, k := range c.Keys {
		w.SetText(a.fields[300+i], keyName(k))
	}
	w.SetText(a.fields[fStatus], "已恢复默认值，点击“保存设置”生效。")
}

func settingsProc(hwnd uintptr, msg uint32, wp, lp uintptr) uintptr {
	a := app
	switch msg {
	case w.WM_NCHITTEST:
		r := w.Bounds(hwnd)
		y := int(int16((lp>>16)&0xffff)) - int(r.Top)
		if y < a.spx(88) {
			return 2
		}
		return 1
	case w.WM_ERASEBKGND:
		w.Fill(wp, w.Client(hwnd), w.RGB(247, 247, 242))
		return 1
	case 0x138, 0x133: // WM_CTLCOLORSTATIC / WM_CTLCOLOREDIT
		w.G("SetTextColor", wp, w.RGB(43, 52, 46))
		w.G("SetBkMode", wp, 1)
		if msg == 0x133 {
			w.G("SetBkColor", wp, w.RGB(255, 255, 255))
			return w.G("GetStockObject", 0)
		}
		w.G("SetDCBrushColor", wp, w.RGB(247, 247, 242))
		return w.G("GetStockObject", 18)
	case w.WM_COMMAND:
		id := int(wp & 0xffff)
		if wp>>16 == 0 {
			switch id {
			case fSave, 1:
				a.applySettings()
			case fCancel, 2:
				a.closeSettings()
			case fReset:
				a.resetSettings()
			}
		}
		return 0
	case w.WM_CLOSE:
		a.closeSettings()
		return 0
	case w.WM_HOTKEY:
		if int(wp) == 100+actBoss {
			a.toggle()
		}
		return 0
	case w.WM_KEYDOWN, w.WM_SYSKEYDOWN:
		if uint32(wp) == a.cfg.Keys[actBoss].Key && currentMods() == a.cfg.Keys[actBoss].Mods {
			a.toggle()
			return 0
		}
	}
	return w.U("DefWindowProcW", hwnd, uintptr(msg), wp, lp)
}
func keyProc(hwnd uintptr, msg uint32, wp, lp uintptr) uintptr {
	a := app
	old := a.editOld[hwnd]
	id := int(w.U("GetDlgCtrlID", hwnd)) - 300
	switch msg {
	case 0x87:
		return 0x1 | 0x4 // WM_GETDLGCODE: capture arrows and combinations.
	case w.WM_SETFOCUS:
		a.capturing = id
		w.U("SendMessageW", hwnd, 0xb1, 0, ^uintptr(0))
	case w.WM_KILLFOCUS:
		a.capturing = -1
	case w.WM_KEYDOWN, w.WM_SYSKEYDOWN:
		if wp == 0x09 {
			next := w.U("GetNextDlgTabItem", a.dialog, hwnd, uintptr((currentMods()>>2)&1))
			w.U("SetFocus", next)
			return 0
		}
		if wp != 0x10 && wp != 0x11 && wp != 0x12 && wp != 0x5b && wp != 0x5c && id >= 0 && id < 5 {
			a.draftKeys[id] = Hotkey{uint32(wp), currentMods()}
			w.SetText(hwnd, keyName(a.draftKeys[id]))
			w.U("SendMessageW", hwnd, 0xb1, 0, ^uintptr(0))
			w.SetText(a.fields[fStatus], "点击“保存设置”生效。如果按键已被占用，会提示修改。")
		}
		return 0
	case w.WM_CHAR, 0x106:
		return 0
	case 0x82:
		delete(a.editOld, hwnd) // WM_NCDESTROY
	}
	if old != 0 {
		return w.U("CallWindowProcW", old, hwnd, uintptr(msg), wp, lp)
	}
	return w.U("DefWindowProcW", hwnd, uintptr(msg), wp, lp)
}
