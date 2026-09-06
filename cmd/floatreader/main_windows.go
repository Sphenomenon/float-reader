package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"

	"floatreader/internal/reader"
	w "floatreader/internal/win32"
)

const appTitle = "隅读 · Float Reader"
const mainClass = "FloatReader.Main.v1"
const settingsClass = "FloatReader.Settings.v1"
const trayMessage = w.WM_APP + 1
const (
	actNext = iota
	actPrevious
	actBoss
	actImport
	actSettings
)

var app *App
var mainProcPtr = syscall.NewCallback(mainProc)
var settingsProcPtr = syscall.NewCallback(settingsProc)
var keyProcPtr = syscall.NewCallback(keyProc)
var hideOwnedProcPtr = syscall.NewCallback(func(hwnd, lp uintptr) uintptr {
	if hwnd == app.hwnd {
		return 1
	}
	for owner := w.U("GetWindow", hwnd, 4); owner != 0; owner = w.U("GetWindow", owner, 4) {
		if owner == app.hwnd {
			if w.U("IsWindowVisible", hwnd) != 0 {
				app.hiddenOwned = append(app.hiddenOwned, hwnd)
				w.U("ShowWindow", hwnd, w.SW_HIDE)
			}
			break
		}
	}
	return 1
})

type Palette struct{ BG, Ink, Muted, Accent, Soft, Line uintptr }

var palettes = []Palette{
	{w.RGB(247, 244, 236), w.RGB(43, 52, 46), w.RGB(121, 128, 116), w.RGB(51, 105, 78), w.RGB(232, 236, 222), w.RGB(221, 225, 213)},
	{w.RGB(30, 36, 35), w.RGB(222, 226, 215), w.RGB(145, 157, 145), w.RGB(157, 197, 153), w.RGB(47, 57, 51), w.RGB(62, 73, 64)},
	{w.RGB(239, 244, 247), w.RGB(46, 58, 69), w.RGB(116, 133, 146), w.RGB(66, 112, 145), w.RGB(222, 233, 239), w.RGB(209, 222, 231)},
}

type App struct {
	hwnd, instance, icon, dialog           uintptr
	cfg                                    Config
	configPath                             string
	dpi                                    int
	fontFace                               string
	settingsDPI                            int
	settingsFont                           uintptr
	hiddenOwned                            []uintptr
	bodyFont, uiFont, smallFont, titleFont uintptr
	text                                   []rune
	pages                                  []reader.Page
	page, anchor                           int
	path, bookName, encoding               string
	hidden, modal, sizing, dirty           bool
	resizeChanged                          bool
	startSize                              w.Size
	trayAdded                              bool
	taskbarCreated                         uint32
	toast                                  string
	toastUntil                             uint64
	fields                                 map[int]uintptr
	editOld                                map[uintptr]uintptr
	draftKeys                              [5]Hotkey
	capturing                              int
	settingsCapacity                       int
	charEdited                             bool
	smoke                                  *Smoke
}

func main() {
	runtime.LockOSThread()
	if p := w.User.NewProc("SetProcessDpiAwarenessContext"); p.Find() == nil {
		p.Call(^uintptr(3))
	} else {
		w.U("SetProcessDPIAware")
	}
	base, err := os.UserConfigDir()
	if err != nil {
		base = os.TempDir()
	}
	path := filepath.Join(base, "FloatReader", "settings.json")
	isSmoke := false
	for _, arg := range os.Args[1:] {
		if arg == "--smoke-test" {
			isSmoke = true
			path = filepath.Join(filepath.Dir(os.Args[0]), "smoke-config.json")
		}
	}
	app = &App{cfg: loadConfig(path), configPath: path, dpi: 96, capturing: -1, editOld: map[uintptr]uintptr{}}
	if isSmoke {
		app.cfg = defaults()
		app.smoke = &Smoke{}
		app.smoke.prepareFont(app)
	}
	app.instance = w.K("GetModuleHandleW", 0)
	mutexName := "Local\\FloatReader.SingleInstance.v1"
	if isSmoke {
		mutexName += ".Smoke"
	}
	mutex, _, mutexErr := w.Kernel.NewProc("CreateMutexW").Call(0, 0, uintptr(unsafe.Pointer(w.Str(mutexName))))
	if mutex != 0 {
		defer w.K("CloseHandle", mutex)
	}
	if mutexErr == syscall.Errno(183) {
		existing := w.U("FindWindowW", uintptr(unsafe.Pointer(w.Str(mainClass))), 0)
		if existing != 0 {
			w.U("PostMessageW", existing, w.WM_APP+2, 0, 0)
		}
		return
	}
	if p := w.User.NewProc("GetDpiForSystem"); p.Find() == nil {
		r, _, _ := p.Call()
		if r > 0 {
			app.dpi = int(r)
		}
	}
	app.icon = w.U("LoadIconW", app.instance, 1)
	if app.icon == 0 {
		app.icon = w.U("LoadIconW", 0, 32512)
	}
	for _, entry := range []struct {
		name string
		proc uintptr
	}{{mainClass, mainProcPtr}, {settingsClass, settingsProcPtr}} {
		wc := w.WndClass{Size: uint32(unsafe.Sizeof(w.WndClass{})), Style: 8, Proc: entry.proc, Instance: app.instance, Icon: app.icon, IconSmall: app.icon, Cursor: w.U("LoadCursorW", 0, 32512), Class: w.Str(entry.name)}
		if w.U("RegisterClassExW", uintptr(unsafe.Pointer(&wc))) == 0 {
			w.Message(0, "窗口初始化失败。", appTitle, 0x10)
			return
		}
	}
	width, height := app.px(app.cfg.Width), app.px(app.cfg.Height)
	work := w.WorkArea(0)
	x, y := int(work.Right)-width-app.px(48), int(work.Top)+app.px(72)
	if app.cfg.Positioned {
		x, y = app.cfg.X, app.cfg.Y
	}
	app.hwnd = w.U("CreateWindowExW", w.WS_EX_TOPMOST|w.WS_EX_TOOLWINDOW|w.WS_EX_LAYERED, uintptr(unsafe.Pointer(w.Str(mainClass))), uintptr(unsafe.Pointer(w.Str(appTitle))), w.WS_POPUP|w.WS_THICKFRAME, w.Signed(x), w.Signed(y), uintptr(width), uintptr(height), 0, 0, app.instance, 0)
	if app.hwnd == 0 {
		w.Message(0, "无法创建阅读窗口。", appTitle, 0x10)
		return
	}
	app.rebuildFonts()
	app.applyAppearance()
	app.ensureOnScreen()
	app.taskbarCreated = uint32(w.U("RegisterWindowMessageW", uintptr(unsafe.Pointer(w.Str("TaskbarCreated")))))
	app.addTray()
	w.S("DragAcceptFiles", app.hwnd, 1)
	if err := app.registerKeys(app.cfg); err != nil {
		app.cfg.GlobalNav = false
		app.cfg.Keys = defaults().Keys
		fallback := app.registerKeys(app.cfg)
		msg := "上次保存的快捷键被占用，已恢复默认快捷键。\n\n" + err.Error()
		if fallback != nil {
			msg += "\n\n默认老板键也被占用。请在设置中换一个组合；仍可通过托盘菜单显示窗口。"
		}
		w.Message(app.hwnd, msg, "快捷键需要调整", 0x30)
	}
	if len(app.cfg.Recent) > 0 {
		mark := app.cfg.Recent[0]
		app.openBook(mark.Path, mark.Offset, false)
	}
	for _, arg := range os.Args[1:] {
		if !strings.HasPrefix(arg, "--") {
			app.openBook(arg, 0, true)
			break
		}
	}
	w.U("ShowWindow", app.hwnd, w.SW_SHOW)
	w.U("UpdateWindow", app.hwnd)
	w.U("SetTimer", app.hwnd, 1, 1000, 0)
	if isSmoke {
		w.U("SetTimer", app.hwnd, 9, 450, 0)
	}
	var msg w.Msg
	for {
		r := int32(w.U("GetMessageW", uintptr(unsafe.Pointer(&msg)), 0, 0, 0))
		if r <= 0 {
			break
		}
		if app.dialog != 0 && w.U("IsDialogMessageW", app.dialog, uintptr(unsafe.Pointer(&msg))) != 0 {
			continue
		}
		w.U("TranslateMessage", uintptr(unsafe.Pointer(&msg)))
		w.U("DispatchMessageW", uintptr(unsafe.Pointer(&msg)))
	}
	if app.smoke != nil {
		for _, c := range app.smoke.checks {
			if !c.Passed {
				os.Exit(1)
			}
		}
	}
}

func (a *App) px(n int) int    { return n * a.dpi / 96 }
func (a *App) dip(n int) int   { return n * 96 / max(96, a.dpi) }
func (a *App) pal() Palette    { return palettes[a.cfg.Theme] }
func (a *App) lineHeight() int { return a.px(a.cfg.FontSize * a.cfg.LineSpace / 100) }
func (a *App) contentRect() w.Rect {
	r := w.Client(a.hwnd)
	return w.Rect{Left: int32(a.px(28)), Top: int32(a.px(88)), Right: r.Right - int32(a.px(28)), Bottom: r.Bottom - int32(a.px(66))}
}
func (a *App) capacity() int {
	r := a.contentRect()
	return reader.Capacity(r.Width(), r.Height(), a.px(a.cfg.FontSize), a.lineHeight())
}
func (a *App) invalidate() { w.U("InvalidateRect", a.hwnd, 0, 0) }
func (a *App) notify(s string) {
	a.toast = s
	a.toastUntil = uint64(w.K("GetTickCount64")) + 2800
	a.invalidate()
}
func (a *App) newFont(size, weight int) uintptr {
	face := a.fontFace
	if face == "" {
		face = "Microsoft YaHei UI"
	}
	return w.G("CreateFontW", w.Signed(-a.px(size)), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(w.Str(face))))
}
func (a *App) rebuildFonts() {
	for _, f := range []uintptr{a.bodyFont, a.uiFont, a.smallFont, a.titleFont} {
		if f != 0 {
			w.G("DeleteObject", f)
		}
	}
	a.bodyFont = a.newFont(a.cfg.FontSize, 400)
	a.uiFont = a.newFont(14, 400)
	a.smallFont = a.newFont(12, 400)
	a.titleFont = a.newFont(22, 600)
}
func (a *App) applyAppearance() {
	w.U("SetLayeredWindowAttributes", a.hwnd, 0, uintptr(a.cfg.Opacity*255/100), 2)
	// Rounded corners where Windows 11 supports them; no standard title bar.
	v := uint32(2)
	p := w.Dwm.NewProc("DwmSetWindowAttribute")
	if p.Find() == nil {
		p.Call(a.hwnd, 33, uintptr(unsafe.Pointer(&v)), 4)
	}
}
func (a *App) ensureOnScreen() {
	r := w.Bounds(a.hwnd)
	work := w.WorkArea(a.hwnd)
	width, height := min(r.Width(), work.Width()), min(r.Height(), work.Height())
	x := clamp(int(r.Left), int(work.Left), int(work.Right)-width)
	y := clamp(int(r.Top), int(work.Top), int(work.Bottom)-height)
	w.U("SetWindowPos", a.hwnd, w.Topmost, w.Signed(x), w.Signed(y), uintptr(width), uintptr(height), w.SWP_NOACTIVATE)
}
func (a *App) paginate() {
	if a.bodyFont == 0 {
		return
	}
	r := a.contentRect()
	dc := w.U("GetDC", a.hwnd)
	old := w.G("SelectObject", dc, a.bodyFont)
	fit := func(rs []rune, width int) int {
		if len(rs) == 0 {
			return 0
		}
		u := utf16.Encode(rs)
		var fit int32
		var size w.Size
		ok := w.G("GetTextExtentExPointW", dc, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)), uintptr(width), uintptr(unsafe.Pointer(&fit)), 0, uintptr(unsafe.Pointer(&size)))
		if ok == 0 {
			return min(len(rs), max(1, width/a.px(a.cfg.FontSize)))
		}
		count, units := 0, 0
		for _, r := range rs {
			n := 1
			if r > 0xffff {
				n = 2
			}
			if units+n > int(fit) {
				break
			}
			units += n
			count++
		}
		return count
	}
	a.pages = reader.Paginate(a.text, r.Width(), max(1, r.Height()/a.lineHeight()), a.cfg.CharLimit, fit)
	w.G("SelectObject", dc, old)
	w.U("ReleaseDC", a.hwnd, dc)
	a.page = reader.PageAt(a.pages, a.anchor)
	a.invalidate()
}
func (a *App) turn(delta int) {
	if a.modal || a.hidden {
		return
	}
	if len(a.text) == 0 {
		a.notify("先导入一本 TXT 小说")
		return
	}
	n := clamp(a.page+delta, 0, len(a.pages)-1)
	if n == a.page {
		if delta > 0 {
			a.notify("已经读到最后一页")
		} else {
			a.notify("已经是第一页")
		}
		return
	}
	a.page = n
	a.anchor = a.pages[n].Start
	a.dirty = true
	a.toast = ""
	a.invalidate()
}
func (a *App) save() {
	r := w.Bounds(a.hwnd)
	a.cfg.X = int(r.Left)
	a.cfg.Y = int(r.Top)
	a.cfg.Positioned = true
	a.cfg.Width = a.dip(r.Width())
	a.cfg.Height = a.dip(r.Height())
	if a.path != "" {
		marks := []BookMark{{a.path, a.anchor}}
		for _, m := range a.cfg.Recent {
			if m.Path != a.path && len(marks) < 8 {
				marks = append(marks, m)
			}
		}
		a.cfg.Recent = marks
	}
	if err := writeConfig(a.configPath, a.cfg); err != nil {
		a.notify("保存失败，请检查配置目录权限")
	} else {
		a.dirty = false
	}
}
func (a *App) toggle() {
	if a.hidden {
		a.hidden = false
		a.ensureOnScreen()
		w.U("ShowWindow", a.hwnd, w.SW_SHOW)
		for _, h := range a.hiddenOwned {
			if w.U("IsWindow", h) != 0 {
				w.U("ShowWindow", h, w.SW_SHOW)
			}
		}
		a.hiddenOwned = nil
		a.restoreNavigation()
		if a.dialog != 0 {
			w.U("ShowWindow", a.dialog, w.SW_SHOW)
			w.U("SetForegroundWindow", a.dialog)
		} else {
			w.U("SetForegroundWindow", a.hwnd)
		}
	} else {
		a.save()
		a.hidden = true
		w.U("UnregisterHotKey", a.hwnd, 100+actNext)
		w.U("UnregisterHotKey", a.hwnd, 100+actPrevious)
		a.hiddenOwned = nil
		w.U("EnumThreadWindows", w.K("GetCurrentThreadId"), hideOwnedProcPtr, 0)
		if a.dialog != 0 {
			w.U("ShowWindow", a.dialog, w.SW_HIDE)
		}
		w.U("ShowWindow", a.hwnd, w.SW_HIDE)
	}
}

func decodeText(data []byte) (string, string, error) {
	if len(data) >= 2 && (data[0] == 0xff && data[1] == 0xfe || data[0] == 0xfe && data[1] == 0xff) {
		if len(data)%2 != 0 {
			return "", "", fmt.Errorf("UTF-16 文件长度异常，可能未下载完整")
		}
		u := make([]uint16, (len(data)-2)/2)
		le := data[0] == 0xff
		for i := range u {
			if le {
				u[i] = binary.LittleEndian.Uint16(data[2+i*2:])
			} else {
				u[i] = binary.BigEndian.Uint16(data[2+i*2:])
			}
		}
		return string(utf16.Decode(u)), "UTF-16", nil
	}
	if utf8.Valid(data) {
		return string(data), "UTF-8", nil
	}
	if len(data) == 0 {
		return "", "UTF-8", nil
	}
	for _, cp := range []uintptr{54936, 936} {
		n := w.K("MultiByteToWideChar", cp, 8, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)), 0, 0)
		if n == 0 {
			continue
		}
		u := make([]uint16, n)
		w.K("MultiByteToWideChar", cp, 8, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)), uintptr(unsafe.Pointer(&u[0])), n)
		return string(utf16.Decode(u)), "GB18030 / GBK", nil
	}
	return "", "", fmt.Errorf("无法识别文字编码，请将文件另存为 UTF-8 或 GB18030 后再导入")
}
func (a *App) openBook(path string, offset int, report bool) bool {
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		err = fmt.Errorf("选中的是文件夹，请选择 TXT 文件")
	}
	if err == nil && info.Size() > 32*1024*1024 {
		err = fmt.Errorf("文件超过 32 MB，请分卷导入")
	}
	var data []byte
	if err == nil {
		data, err = os.ReadFile(path)
	}
	var s, encoding string
	if err == nil {
		s, encoding, err = decodeText(data)
	}
	if err == nil && strings.TrimSpace(s) == "" {
		err = fmt.Errorf("文件里没有文字")
	}
	if err != nil {
		if report {
			w.Message(a.hwnd, "导入失败：\n"+err.Error(), appTitle, 0x30)
		}
		return false
	}
	if a.path != "" {
		a.save()
	}
	if offset == 0 {
		for _, m := range a.cfg.Recent {
			if strings.EqualFold(m.Path, path) {
				offset = m.Offset
				break
			}
		}
	}
	a.path = path
	a.bookName = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	a.encoding = encoding
	a.text = reader.Normalize(s)
	a.anchor = clamp(offset, 0, max(0, len(a.text)-1))
	a.paginate()
	a.save()
	return true
}
func (a *App) importBook() {
	if a.modal {
		return
	}
	a.modal = true
	buf := make([]uint16, 32768)
	filter := utf16.Encode([]rune("TXT 文本文件 (*.txt)\x00*.txt\x00所有文件 (*.*)\x00*.*\x00\x00"))
	of := w.OpenFile{Size: uint32(unsafe.Sizeof(w.OpenFile{})), Owner: a.hwnd, Filter: &filter[0], FilterIndex: 1, File: &buf[0], MaxFile: uint32(len(buf)), Title: w.Str("导入小说 · 隅读"), Flags: 0x1000 | 0x800 | 0x8 | 0x80000, DefExt: w.Str("txt")}
	ok := w.C("GetOpenFileNameW", uintptr(unsafe.Pointer(&of)))
	a.modal = false
	if ok != 0 {
		a.openBook(syscall.UTF16ToString(buf), 0, true)
	}
}

func mainProc(hwnd uintptr, msg uint32, wp, lp uintptr) uintptr {
	a := app
	if a == nil {
		return w.U("DefWindowProcW", hwnd, uintptr(msg), wp, lp)
	}
	if msg == a.taskbarCreated && a.taskbarCreated != 0 {
		a.trayAdded = false
		a.addTray()
		return 0
	}
	switch msg {
	case w.WM_NCCALCSIZE:
		return 0
	case w.WM_NCHITTEST:
		r := w.Bounds(hwnd)
		x, y := int(int16(lp&0xffff))-int(r.Left), int(int16((lp>>16)&0xffff))-int(r.Top)
		b := a.px(7)
		left, right, top, bottom := x < b, x >= r.Width()-b, y < b, y >= r.Height()-b
		switch {
		case top && left:
			return 13
		case top && right:
			return 14
		case bottom && left:
			return 16
		case bottom && right:
			return 17
		case left:
			return 10
		case right:
			return 11
		case top:
			return 12
		case bottom:
			return 15
		}
		if y < a.px(72) && x < r.Width()-a.px(80) {
			return 2
		}
		return 1
	case w.WM_GETMINMAXINFO:
		m := (*w.MinMaxInfo)(unsafe.Pointer(lp))
		m.MinTrackSize = w.Point{X: int32(a.px(320)), Y: int32(a.px(230))}
		return 0
	case w.WM_ERASEBKGND:
		return 1
	case w.WM_PAINT:
		a.paint(hwnd)
		return 0
	case w.WM_SIZE:
		if a.hwnd != 0 && a.bodyFont != 0 {
			if a.sizing {
				r := w.Client(hwnd)
				if r.Width() != int(a.startSize.CX) || r.Height() != int(a.startSize.CY) {
					a.resizeChanged = true
				}
				if a.resizeChanged {
					w.U("SetTimer", hwnd, 2, 70, 0)
				}
				a.invalidate()
			} else {
				a.paginate()
			}
			a.dirty = true
		}
		return 0
	case w.WM_ENTERSIZEMOVE:
		a.sizing = true
		r := w.Client(hwnd)
		a.startSize = w.Size{CX: int32(r.Width()), CY: int32(r.Height())}
		a.resizeChanged = false
		return 0
	case w.WM_EXITSIZEMOVE:
		a.sizing = false
		w.U("KillTimer", hwnd, 2)
		if a.resizeChanged {
			a.cfg.CharLimit = 0
			a.paginate()
		}
		a.save()
		return 0
	case w.WM_DISPLAYCHANGE:
		a.ensureOnScreen()
		return 0
	case w.WM_DPICHANGED:
		a.dpi = int(wp & 0xffff)
		r := (*w.Rect)(unsafe.Pointer(lp))
		a.rebuildFonts()
		w.U("SetWindowPos", hwnd, 0, w.Signed(int(r.Left)), w.Signed(int(r.Top)), uintptr(r.Width()), uintptr(r.Height()), w.SWP_NOZORDER|w.SWP_NOACTIVATE)
		a.paginate()
		return 0
	case w.WM_TIMER:
		if wp == 9 && a.smoke != nil {
			a.smoke.step(a)
			return 0
		}
		if wp == 2 {
			w.U("KillTimer", hwnd, 2)
			if a.resizeChanged {
				a.cfg.CharLimit = 0
				a.paginate()
			}
			return 0
		}
		if !a.hidden {
			w.U("SetWindowPos", hwnd, w.Topmost, 0, 0, 0, 0, w.SWP_NOSIZE|w.SWP_NOMOVE|w.SWP_NOACTIVATE)
			if a.dialog != 0 {
				w.U("SetWindowPos", a.dialog, w.Topmost, 0, 0, 0, 0, w.SWP_NOSIZE|w.SWP_NOMOVE|w.SWP_NOACTIVATE)
			}
		}
		if a.dirty && !a.sizing {
			a.save()
		}
		if a.toast != "" && uint64(w.K("GetTickCount64")) > a.toastUntil {
			a.toast = ""
			a.invalidate()
		}
		return 0
	case w.WM_KEYDOWN, w.WM_SYSKEYDOWN:
		if a.modal {
			return 0
		}
		if a.handleKey(uint32(wp), currentMods()) {
			return 0
		}
	case w.WM_HOTKEY:
		if a.smoke != nil {
			a.smoke.events = append(a.smoke.events, fmt.Sprintf("hotkey %d hidden=%t modal=%t page=%d", wp, a.hidden, a.modal, a.page))
		}
		id := int(wp) - 100
		if id >= 0 && id < 5 {
			a.action(id)
		}
		return 0
	case w.WM_MOUSEWHEEL:
		if int16((wp>>16)&0xffff) > 0 {
			a.turn(-1)
		} else {
			a.turn(1)
		}
		return 0
	case w.WM_LBUTTONUP:
		x, y := int(int16(lp&0xffff)), int(int16((lp>>16)&0xffff))
		r := w.Client(hwnd)
		if y < a.px(70) && x > r.Width()-a.px(80) {
			a.openSettings()
			return 0
		}
		if len(a.text) == 0 {
			if a.importButton().contains(x, y) {
				a.importBook()
			}
			return 0
		}
		if y > r.Height()-a.px(60) {
			if x < a.px(64) {
				a.turn(-1)
			} else if x > r.Width()-a.px(64) {
				a.turn(1)
			}
		}
		return 0
	case w.WM_RBUTTONUP:
		a.contextMenu(false)
		return 0
	case w.WM_DROPFILES:
		buf := make([]uint16, 32768)
		w.S("DragQueryFileW", wp, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		w.S("DragFinish", wp)
		if !a.modal {
			a.openBook(syscall.UTF16ToString(buf), 0, true)
		}
		return 0
	case trayMessage:
		if uint32(lp) == w.WM_LBUTTONUP {
			a.toggle()
		} else if uint32(lp) == w.WM_RBUTTONUP {
			a.contextMenu(true)
		}
		return 0
	case w.WM_APP + 2:
		if a.hidden {
			a.toggle()
		} else {
			w.U("SetForegroundWindow", hwnd)
		}
		return 0
	case w.WM_CLOSE:
		a.toggle()
		return 0
	case w.WM_QUERYENDSESSION:
		a.save()
		return 1
	case w.WM_ENDSESSION:
		if wp != 0 {
			a.save()
		}
		return 0
	case w.WM_DESTROY:
		a.save()
		a.removeTray()
		for i := 0; i < 5; i++ {
			w.U("UnregisterHotKey", hwnd, uintptr(100+i))
		}
		w.U("PostQuitMessage", 0)
		return 0
	}
	return w.U("DefWindowProcW", hwnd, uintptr(msg), wp, lp)
}
