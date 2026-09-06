package main

import (
	w "floatreader/internal/win32"
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

var actionNames = [5]string{"下一页", "上一页", "老板键（隐藏 / 显示）", "导入 TXT", "打开设置"}

func currentMods() uint32 {
	var m uint32
	for _, p := range []struct {
		k uintptr
		m uint32
	}{{0x12, 1}, {0x11, 2}, {0x10, 4}, {0x5b, 8}, {0x5c, 8}} {
		if int16(w.U("GetKeyState", p.k)) < 0 {
			m |= p.m
		}
	}
	return m
}
func keyName(k Hotkey) string {
	var parts []string
	if k.Mods&2 != 0 {
		parts = append(parts, "Ctrl")
	}
	if k.Mods&1 != 0 {
		parts = append(parts, "Alt")
	}
	if k.Mods&4 != 0 {
		parts = append(parts, "Shift")
	}
	if k.Mods&8 != 0 {
		parts = append(parts, "Win")
	}
	names := map[uint32]string{0x25: "←", 0x26: "↑", 0x27: "→", 0x28: "↓", 0x20: "Space", 0x0d: "Enter", 0x1b: "Esc", 0x09: "Tab", 0x21: "Page Up", 0x22: "Page Down", 0x23: "End", 0x24: "Home", 0x2d: "Insert", 0x2e: "Delete", 0x08: "Backspace", 0xbc: ",", 0xbe: ".", 0xba: ";", 0xbf: "/", 0xbd: "-", 0xbb: "=", 0xc0: "`", 0xdb: "[", 0xdd: "]", 0xdc: "\\"}
	name := names[k.Key]
	if name == "" {
		if k.Key >= 'A' && k.Key <= 'Z' || k.Key >= '0' && k.Key <= '9' {
			name = string(rune(k.Key))
		} else if k.Key >= 0x70 && k.Key <= 0x87 {
			name = fmt.Sprintf("F%d", k.Key-0x6f)
		} else {
			buf := make([]uint16, 64)
			scan := w.U("MapVirtualKeyW", uintptr(k.Key), 0)
			w.U("GetKeyNameTextW", scan<<16, uintptr(unsafe.Pointer(&buf[0])), 64)
			name = syscall.UTF16ToString(buf)
			if name == "" {
				name = fmt.Sprintf("Key %d", k.Key)
			}
		}
	}
	return strings.Join(append(parts, name), " + ")
}
func validateKeys(c Config) error {
	for i, k := range c.Keys {
		if k.Key == 0 || k.Key == 0x10 || k.Key == 0x11 || k.Key == 0x12 || k.Key == 0x5b || k.Key == 0x5c {
			return fmt.Errorf("%s：请按一个完整的组合键", actionNames[i])
		}
		if k.Key == 0x7b {
			return fmt.Errorf("F12 被 Windows 调试器保留，请使用其他按键")
		}
		if k.Key == 0x09 || (k.Mods&1 != 0 && k.Key == 0x73) || (k.Mods&3 == 3 && k.Key == 0x2e) || k.Mods&8 != 0 || (k.Key == 0x1b && k.Mods&3 != 0) || (k.Key == 0x20 && k.Mods == 1) {
			return fmt.Errorf("%s 属于系统保留或导航组合，请使用 Ctrl、Alt、Shift 或 F1–F11", keyName(k))
		}
		if i == actBoss && k.Mods == 0 && (k.Key < 0x70 || k.Key > 0x7a) {
			return fmt.Errorf("老板键是全局快捷键，请加 Ctrl / Alt / Shift，或使用 F1–F11，避免影响日常输入")
		}
		for j := 0; j < i; j++ {
			if c.Keys[j] == k {
				return fmt.Errorf("%s 和 %s 使用了相同的快捷键", actionNames[j], actionNames[i])
			}
		}
	}
	return nil
}
func (a *App) registerKeys(c Config) error {
	for i := 0; i < 5; i++ {
		w.U("UnregisterHotKey", a.hwnd, uintptr(100+i))
	}
	var registered []int
	for i, k := range c.Keys {
		if i != actBoss && !(c.GlobalNav && (i == actNext || i == actPrevious)) {
			continue
		}
		if w.U("RegisterHotKey", a.hwnd, uintptr(100+i), uintptr(k.Mods|w.MOD_NOREPEAT), uintptr(k.Key)) == 0 {
			for _, j := range registered {
				w.U("UnregisterHotKey", a.hwnd, uintptr(100+j))
			}
			return fmt.Errorf("%s 无法使用 %s，可能已被其他软件占用，请换一组按键", actionNames[i], keyName(k))
		}
		registered = append(registered, i)
	}
	return nil
}

func (a *App) restoreNavigation() {
	if !a.cfg.GlobalNav {
		return
	}
	for _, i := range []int{actNext, actPrevious} {
		k := a.cfg.Keys[i]
		if w.U("RegisterHotKey", a.hwnd, uintptr(100+i), uintptr(k.Mods|w.MOD_NOREPEAT), uintptr(k.Key)) == 0 {
			w.U("UnregisterHotKey", a.hwnd, 100+actNext)
			w.U("UnregisterHotKey", a.hwnd, 100+actPrevious)
			a.cfg.GlobalNav = false
			a.dirty = true
			a.notify("全局翻页键被占用，请在设置中重新指定")
			return
		}
	}
}
func (a *App) handleKey(key, mods uint32) bool {
	for i, k := range a.cfg.Keys {
		if k.Key == key && k.Mods == mods {
			// Registered hotkeys are dispatched by WM_HOTKEY, not a second time here.
			if i != actBoss && !(a.cfg.GlobalNav && (i == actNext || i == actPrevious)) {
				a.action(i)
			}
			return true
		}
	}
	if mods == 0 && !a.cfg.GlobalNav {
		if key == w.VK_DOWN && a.cfg.Keys[actNext] == (Hotkey{w.VK_RIGHT, 0}) {
			a.turn(1)
			return true
		}
		if key == w.VK_UP && a.cfg.Keys[actPrevious] == (Hotkey{w.VK_LEFT, 0}) {
			a.turn(-1)
			return true
		}
	}
	return false
}
func (a *App) action(id int) {
	if id == actBoss {
		a.toggle()
		return
	}
	if a.hidden || a.modal {
		return
	}
	switch id {
	case actNext:
		a.turn(1)
	case actPrevious:
		a.turn(-1)
	case actImport:
		a.importBook()
	case actSettings:
		a.openSettings()
	}
}
func (a *App) addTray() {
	n := w.NotifyIcon{Size: uint32(unsafe.Sizeof(w.NotifyIcon{})), Hwnd: a.hwnd, ID: 1, Flags: 1 | 2 | 4, Callback: trayMessage, Icon: a.icon}
	copy(n.Tip[:], w.UTF16("隅读 · 点击显示/隐藏，右键打开菜单"))
	a.trayAdded = w.S("Shell_NotifyIconW", 0, uintptr(unsafe.Pointer(&n))) != 0
}
func (a *App) removeTray() {
	if a.trayAdded {
		n := w.NotifyIcon{Size: uint32(unsafe.Sizeof(w.NotifyIcon{})), Hwnd: a.hwnd, ID: 1}
		w.S("Shell_NotifyIconW", 2, uintptr(unsafe.Pointer(&n)))
	}
}
func (a *App) contextMenu(tray bool) {
	menu := w.U("CreatePopupMenu")
	defer w.U("DestroyMenu", menu)
	add := func(id int, s string) { w.U("AppendMenuW", menu, 0, uintptr(id), uintptr(unsafe.Pointer(w.Str(s)))) }
	sep := func() { w.U("AppendMenuW", menu, 0x800, 0, 0) }
	if a.hidden {
		add(1, "显示阅读窗口")
	} else {
		add(1, "隐藏阅读窗口\t"+keyName(a.cfg.Keys[actBoss]))
	}
	add(2, "导入 TXT…\t"+keyName(a.cfg.Keys[actImport]))
	add(3, "设置…")
	sep()
	add(4, "上一页\t"+keyName(a.cfg.Keys[actPrevious]))
	add(5, "下一页\t"+keyName(a.cfg.Keys[actNext]))
	if len(a.cfg.Recent) > 0 {
		sep()
		for i, m := range a.cfg.Recent {
			add(20+i, "最近阅读 · "+shortPath(m.Path))
		}
	}
	sep()
	add(6, "退出隅读")
	var p w.Point
	w.U("GetCursorPos", uintptr(unsafe.Pointer(&p)))
	w.U("SetForegroundWindow", a.hwnd)
	id := int(w.U("TrackPopupMenu", menu, 0x100|2, w.Signed(int(p.X)), w.Signed(int(p.Y)), 0, a.hwnd, 0))
	w.U("PostMessageW", a.hwnd, 0, 0, 0)
	switch id {
	case 1:
		a.toggle()
	case 2:
		if a.hidden {
			a.toggle()
		}
		a.importBook()
	case 3:
		if a.hidden {
			a.toggle()
		}
		a.openSettings()
	case 4:
		a.turn(-1)
	case 5:
		a.turn(1)
	case 6:
		if a.dialog != 0 {
			a.closeSettings()
		}
		w.U("DestroyWindow", a.hwnd)
	default:
		if id >= 20 && id < 20+len(a.cfg.Recent) {
			if a.hidden {
				a.toggle()
			}
			m := a.cfg.Recent[id-20]
			a.openBook(m.Path, m.Offset, true)
		}
	}
}
func shortPath(s string) string {
	s = strings.ReplaceAll(s, "\\", "/")
	parts := strings.Split(s, "/")
	return parts[len(parts)-1]
}
