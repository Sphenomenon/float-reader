package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	w "floatreader/internal/win32"
)

// --smoke-test runs the real native windows and OS hotkeys under Windows / Wine.
// Results and screenshots go beside the executable in qa/.
type Smoke struct {
	phase      int
	checks     []Check
	before     int
	dir        string
	dummy      uintptr
	fileDialog uintptr
	events     []string
}

func (s *Smoke) prepareFont(a *App) {
	// Wine on Linux does not ship Microsoft's Chinese fonts. Load the host's
	// Noto font for QA only; production Windows uses Microsoft YaHei UI.
	path := `Z:\usr\share\fonts\google-noto-sans-cjk-fonts\NotoSansCJK-Regular.ttc`
	if w.G("AddFontResourceExW", uintptr(unsafe.Pointer(w.Str(path))), 0x10, 0) > 0 {
		a.fontFace = "Noto Sans CJK SC"
	}
}

type Check struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail,omitempty"`
}

func (s *Smoke) check(name string, ok bool, detail ...any) {
	s.checks = append(s.checks, Check{name, ok, fmt.Sprint(detail...)})
}
func (s *Smoke) shot(hwnd uintptr, name string) {
	err := capture(hwnd, filepath.Join(s.dir, name+".png"))
	s.check("screenshot: "+name, err == nil, err)
}
func press(k Hotkey) {
	mods := []struct {
		bit uint32
		vk  uintptr
	}{{2, 0x11}, {1, 0x12}, {4, 0x10}, {8, 0x5b}}
	for _, m := range mods {
		if k.Mods&m.bit != 0 {
			w.U("keybd_event", m.vk, 0, 0, 0)
		}
	}
	flags := uintptr(0)
	if k.Key >= 0x21 && k.Key <= 0x2e {
		flags = 1
	}
	w.U("keybd_event", uintptr(k.Key), 0, flags, 0)
	w.U("keybd_event", uintptr(k.Key), 0, flags|2, 0)
	for i := len(mods) - 1; i >= 0; i-- {
		m := mods[i]
		if k.Mods&m.bit != 0 {
			w.U("keybd_event", m.vk, 0, 2, 0)
		}
	}
}
func (s *Smoke) step(a *App) {
	switch s.phase {
	case 0:
		s.dir = filepath.Join(filepath.Dir(os.Args[0]), "qa")
		os.MkdirAll(s.dir, 0755)
		w.U("SetWindowPos", a.hwnd, w.Topmost, 80, 80, uintptr(a.px(460)), uintptr(a.px(580)), w.SWP_NOACTIVATE)
		s.check("topmost style", w.U("GetWindowLongPtrW", a.hwnd, w.Signed(-20))&w.WS_EX_TOPMOST != 0)
		s.check("no title bar", w.U("GetWindowLongPtrW", a.hwnd, w.Signed(-16))&0xc00000 == 0)
		s.shot(a.hwnd, "01-welcome")
		text := "山海来信\n\n第一章  风从山谷来\n\n九月的第一场雨，落在天亮以前。\n\n林予推开窗，远处的山脊像一封尚未拆开的信。巷口的早餐铺已经亮了灯，热气从蒸笼里升起来，把清晨的街道染得柔软。\n\n她把昨夜读到一半的书放进帆布包，沿着石阶往下走。今天没有什么特别的安排，她只是想去看看，那条传说中通往海边的小路。\n\n“等一下。”身后有人叫她。\n\n老人递来一把蓝色的伞，说：“山里的天气，和人心一样，偶尔也会改变主意。”\n\n"
		text += strings.Repeat("风吹过树梢，纸页轻轻翻动。故事还在继续，每一步都有新的风景。The quiet road leads towards the sea. 🌿\n\n", 120)
		path := filepath.Join(s.dir, "山海来信.txt")
		os.WriteFile(path, []byte(text), 0600)
		s.check("UTF-8 import", a.openBook(path, 0, false), "pages=", len(a.pages))
		gbk, _, err := decodeText([]byte{0xd6, 0xd0, 0xce, 0xc4, 0xd0, 0xa1, 0xcb, 0xb5})
		s.check("GBK import", err == nil && gbk == "中文小说", gbk)
		ut, _, err := decodeText([]byte{0xff, 0xfe, 0x2d, 0x4e, 0x87, 0x65})
		s.check("UTF-16 import", err == nil && ut == "中文", ut)
		s.check("empty file rejection", !a.openBook(filepath.Join(s.dir, "missing.txt"), 0, false))
	case 1:
		s.shot(a.hwnd, "02-reading")
		s.before = a.page
		w.U("SendMessageW", a.hwnd, w.WM_KEYDOWN, uintptr(a.cfg.Keys[actNext].Key), 0)
		s.check("native right-arrow input advances exactly one page", a.page == s.before+1, "page=", a.page)
	case 2:
		a.page = min(1, len(a.pages)-1)
		a.anchor = a.pages[a.page].Start
		w.U("SendMessageW", a.hwnd, w.WM_KEYDOWN, w.VK_UP, 0)
		s.check("native up-arrow input returns to previous page", a.page == 0, "page=", a.page)
	case 3:
		press(a.cfg.Keys[actBoss])
	case 4:
		s.check("global boss key hides native window", a.hidden && w.U("IsWindowVisible", a.hwnd) == 0)
		s.before = a.page
		a.turn(1)
		s.check("hidden window does not change page", a.page == s.before)
		press(a.cfg.Keys[actBoss])
	case 5:
		s.check("same global boss key restores native window", !a.hidden && w.U("IsWindowVisible", a.hwnd) != 0)
		a.openSettings()
	case 6:
		s.shot(a.dialog, "03-settings")
		w.U("SendMessageW", a.fields[300], w.WM_KEYDOWN, 0x75, 0)
		s.check("shortcut recorder captures keyboard input", a.draftKeys[0].Key == 0x75 && w.Text(a.fields[300]) == "F6")
		a.draftKeys = a.cfg.Keys
		w.SetText(a.fields[300], keyName(a.cfg.Keys[0]))
		press(a.cfg.Keys[actBoss])
	case 7:
		s.check("boss key also hides settings", a.hidden && w.U("IsWindowVisible", a.dialog) == 0)
		press(a.cfg.Keys[actBoss])
	case 8:
		s.check("boss key restores settings", !a.hidden && w.U("IsWindowVisible", a.dialog) != 0)
		w.SetText(a.fields[fWidth], "invalid")
		s.check("invalid dimensions rejected", !a.applySettings() && a.dialog != 0)
		w.SetText(a.fields[fWidth], "500")
		w.SetText(a.fields[fHeight], "530")
		w.SetText(a.fields[fFont], "22")
		s.check("numeric dimensions and font saved", a.applySettings())
		r := w.Bounds(a.hwnd)
		s.check("width height applied", a.dip(r.Width()) == 500 && a.dip(r.Height()) == 530, r)
		a.openSettings()
		w.SetText(a.fields[fChars], "80")
		s.check("editable page character count", a.applySettings())
	case 9:
		ok := true
		for _, p := range a.pages {
			if p.End-p.Start > 80 {
				ok = false
			}
		}
		s.check("character cap never overflows", ok && a.cfg.CharLimit == 80)
		w.U("SendMessageW", a.hwnd, w.WM_ENTERSIZEMOVE, 0, 0)
		rmove := w.Bounds(a.hwnd)
		w.U("SetWindowPos", a.hwnd, 0, w.Signed(int(rmove.Left)+4), w.Signed(int(rmove.Top)+4), 0, 0, w.SWP_NOSIZE|w.SWP_NOZORDER|w.SWP_NOACTIVATE)
		w.U("SendMessageW", a.hwnd, w.WM_EXITSIZEMOVE, 0, 0)
		s.check("moving the window preserves manual character limit", a.cfg.CharLimit == 80)
		a.page = min(3, len(a.pages)-1)
		a.anchor = a.pages[a.page].Start
		s.before = a.anchor
		w.U("SendMessageW", a.hwnd, w.WM_ENTERSIZEMOVE, 0, 0)
		w.U("SetWindowPos", a.hwnd, 0, 0, 0, uintptr(a.px(430)), uintptr(a.px(540)), w.SWP_NOMOVE|w.SWP_NOZORDER)
		w.U("SendMessageW", a.hwnd, w.WM_EXITSIZEMOVE, 0, 0)
		s.check("resize restores adaptive pagination", a.cfg.CharLimit == 0 && a.pages[a.page].Start <= s.before && a.pages[a.page].End > s.before)
		r := w.Bounds(a.hwnd)
		lp := uintptr(uint16(r.Left+2)) | uintptr(uint16(r.Top+2))<<16
		s.check("border drag resize hit testing", w.U("SendMessageW", a.hwnd, w.WM_NCHITTEST, 0, lp) == 13)
		lp = uintptr(uint16(r.Left+70)) | uintptr(uint16(r.Top+30))<<16
		s.check("header drag move hit testing", w.U("SendMessageW", a.hwnd, w.WM_NCHITTEST, 0, lp) == 2)
		a.openSettings()
		a.draftKeys[0] = a.draftKeys[1]
		s.check("duplicate shortcuts rejected", !a.applySettings())
		a.draftKeys = a.cfg.Keys
		w.U("RegisterHotKey", a.hwnd, 999, 3, 'K')
		a.draftKeys[actBoss] = Hotkey{'K', 3}
		s.check("occupied global shortcut rejected and rolled back", !a.applySettings() && a.cfg.Keys[actBoss] == defaults().Keys[actBoss])
		w.U("UnregisterHotKey", a.hwnd, 999)
		a.draftKeys = a.cfg.Keys
		a.draftKeys[actNext] = Hotkey{w.VK_RIGHT, 6}
		a.draftKeys[actPrevious] = Hotkey{w.VK_LEFT, 6}
		w.U("SendMessageW", a.fields[fGlobal], 0xf1, 1, 0)
		w.U("SendMessageW", a.fields[fTheme], 0x14e, 1, 0)
		s.check("custom global navigation settings", a.applySettings())
	case 10:
		s.shot(a.hwnd, "04-dark-adaptive")
		s.dummy = w.U("CreateWindowExW", 0, uintptr(unsafe.Pointer(w.Str("STATIC"))), uintptr(unsafe.Pointer(w.Str("QA foreground window"))), 0x10cf0000, 30, 30, 300, 180, 0, 0, a.instance, 0)
		w.U("ShowWindow", s.dummy, w.SW_SHOW)
		w.U("SetForegroundWindow", s.dummy)
		s.before = a.anchor
		press(a.cfg.Keys[actNext])
	case 11:
		s.check("custom global next works with another window focused", a.anchor > s.before, "offset=", a.anchor, " previous=", s.before)
		press(a.cfg.Keys[actBoss])
	case 12:
		s.check("boss key works while another window is focused", a.hidden)
		press(a.cfg.Keys[actNext])
		s.before = a.page
	case 13:
		s.check("global navigation while hidden cannot advance", s.before == a.page)
		press(a.cfg.Keys[actBoss])
	case 14:
		w.U("DestroyWindow", s.dummy)
		s.check("restore after external focus", !a.hidden)
		a.save()
		saved := loadConfig(a.configPath)
		s.check("preferences and reading progress persist", saved.Keys == a.cfg.Keys && len(saved.Recent) > 0 && saved.Recent[0].Offset == a.anchor)
		// Restore the polished default reading view for the final screenshot.
		c := defaults()
		a.cfg = c
		a.rebuildFonts()
		w.U("SetWindowPos", a.hwnd, w.Topmost, 0, 0, uintptr(a.px(c.Width)), uintptr(a.px(c.Height)), w.SWP_NOMOVE)
		a.anchor = 0
		a.paginate()
		a.applyAppearance()
	case 15:
		s.shot(a.hwnd, "05-final")
		a.registerKeys(a.cfg)
		s.phase = 16
		a.importBook()
		return
	case 16:
		s.fileDialog = w.U("GetLastActivePopup", a.hwnd)
		s.check("native TXT file picker opens", s.fileDialog != a.hwnd && w.U("IsWindowVisible", s.fileDialog) != 0)
		press(a.cfg.Keys[actBoss])
	case 17:
		s.check("boss key hides native file picker", a.hidden && w.U("IsWindowVisible", s.fileDialog) == 0)
		press(a.cfg.Keys[actBoss])
	case 18:
		s.check("boss key restores native file picker", !a.hidden && w.U("IsWindowVisible", s.fileDialog) != 0)
		w.U("PostMessageW", s.fileDialog, w.WM_CLOSE, 0, 0)
	case 19:
		s.check("file picker cancellation restores reader", !a.modal && !a.hidden)
		a.dpi = 192
		a.rebuildFonts()
		w.U("SetWindowPos", a.hwnd, w.Topmost, 0, 0, uintptr(a.px(460)), uintptr(a.px(580)), w.SWP_NOMOVE|w.SWP_NOACTIVATE)
		a.ensureOnScreen()
		a.openSettings()
	case 20:
		work := w.WorkArea(a.hwnd)
		r := w.Bounds(a.dialog)
		save := w.Bounds(a.fields[fSave])
		s.check("settings fit available screen at 200 percent UI scale", r.Width() <= work.Width() && r.Height() <= work.Height() && save.Bottom <= work.Bottom && save.Right <= work.Right, fmt.Sprint(r))
		s.shot(a.dialog, "06-settings-scaled")
		a.closeSettings()
		a.dpi = 96
		a.rebuildFonts()
		w.U("SetWindowPos", a.hwnd, w.Topmost, 0, 0, 460, 580, w.SWP_NOMOVE|w.SWP_NOACTIVATE)
	case 21:
		large := strings.Repeat("山间的风带来一封信，故事沿着小路继续。\n", 50000)
		a.text = []rune(large)
		a.anchor = 0
		started := time.Now()
		a.paginate()
		elapsed := time.Since(started)
		s.check("million-character novel paginates without losing text", len(a.pages) > 1 && a.pages[len(a.pages)-1].End == len(a.text), fmt.Sprintf("%d characters, %d pages, %s", len(a.text), len(a.pages), elapsed.Round(time.Millisecond)))
		path := filepath.Join(s.dir, "empty.txt")
		os.WriteFile(path, nil, 0600)
		s.check("empty TXT rejected without replacing book", !a.openBook(path, 0, false) && len(a.text) == len([]rune(large)))
	case 22:
		data, _ := json.MarshalIndent(s.checks, "", "  ")
		os.WriteFile(filepath.Join(s.dir, "report.json"), data, 0644)
		os.WriteFile(filepath.Join(s.dir, "events.txt"), []byte(strings.Join(s.events, "\n")), 0644)
		w.U("KillTimer", a.hwnd, 9)
		w.U("DestroyWindow", a.hwnd)
	}
	s.phase++
}

type bitmapInfo struct {
	Size                   uint32
	Width, Height          int32
	Planes, BitCount       uint16
	Compression, ImageSize uint32
	X, Y                   int32
	Used, Important        uint32
}

func capture(hwnd uintptr, path string) error {
	if hwnd == 0 {
		return fmt.Errorf("window not available")
	}
	w.U("UpdateWindow", hwnd)
	r := w.Client(hwnd)
	dc := w.U("GetDC", hwnd)
	defer w.U("ReleaseDC", hwnd, dc)
	mem := w.G("CreateCompatibleDC", dc)
	defer w.G("DeleteDC", mem)
	bmp := w.G("CreateCompatibleBitmap", dc, uintptr(r.Width()), uintptr(r.Height()))
	defer w.G("DeleteObject", bmp)
	old := w.G("SelectObject", mem, bmp)
	w.G("BitBlt", mem, 0, 0, uintptr(r.Width()), uintptr(r.Height()), dc, 0, 0, 0xcc0020)
	w.G("SelectObject", mem, old)
	bi := bitmapInfo{Size: uint32(unsafe.Sizeof(bitmapInfo{})), Width: int32(r.Width()), Height: -int32(r.Height()), Planes: 1, BitCount: 32}
	pixels := make([]byte, r.Width()*r.Height()*4)
	if w.G("GetDIBits", mem, bmp, 0, uintptr(r.Height()), uintptr(unsafe.Pointer(&pixels[0])), uintptr(unsafe.Pointer(&bi)), 0) == 0 {
		return fmt.Errorf("GetDIBits failed")
	}
	im := image.NewRGBA(image.Rect(0, 0, r.Width(), r.Height()))
	for y := 0; y < r.Height(); y++ {
		for x := 0; x < r.Width(); x++ {
			i := (y*r.Width() + x) * 4
			im.SetRGBA(x, y, color.RGBA{pixels[i+2], pixels[i+1], pixels[i], 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, im)
}
