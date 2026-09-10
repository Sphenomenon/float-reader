// Package win32 provides the small native API surface used by Float Reader.
package win32

import (
	"sync"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var User = syscall.NewLazyDLL("user32.dll")
var GDI = syscall.NewLazyDLL("gdi32.dll")
var Kernel = syscall.NewLazyDLL("kernel32.dll")
var Shell = syscall.NewLazyDLL("shell32.dll")
var Comdlg = syscall.NewLazyDLL("comdlg32.dll")
var Dwm = syscall.NewLazyDLL("dwmapi.dll")

type procKey struct {
	dll  *syscall.LazyDLL
	name string
}

var procCache sync.Map

func proc(dll *syscall.LazyDLL, name string) *syscall.LazyProc {
	key := procKey{dll, name}
	if p, ok := procCache.Load(key); ok {
		return p.(*syscall.LazyProc)
	}
	p, _ := procCache.LoadOrStore(key, dll.NewProc(name))
	return p.(*syscall.LazyProc)
}

//go:uintptrescapes
func U(name string, args ...uintptr) uintptr { r, _, _ := proc(User, name).Call(args...); return r }

//go:uintptrescapes
func G(name string, args ...uintptr) uintptr { r, _, _ := proc(GDI, name).Call(args...); return r }

//go:uintptrescapes
func K(name string, args ...uintptr) uintptr { r, _, _ := proc(Kernel, name).Call(args...); return r }

//go:uintptrescapes
func S(name string, args ...uintptr) uintptr { r, _, _ := proc(Shell, name).Call(args...); return r }

//go:uintptrescapes
func C(name string, args ...uintptr) uintptr { r, _, _ := proc(Comdlg, name).Call(args...); return r }
func Str(s string) *uint16                   { p, _ := syscall.UTF16PtrFromString(s); return p }
func UTF16(s string) []uint16                { return utf16.Encode([]rune(s)) }
func RGB(r, g, b uint32) uintptr             { return uintptr(r | g<<8 | b<<16) }
func Signed(v int) uintptr                   { return uintptr(int64(v)) }

type Point struct{ X, Y int32 }
type Rect struct{ Left, Top, Right, Bottom int32 }

func (r Rect) Width() int  { return int(r.Right - r.Left) }
func (r Rect) Height() int { return int(r.Bottom - r.Top) }

type Msg struct {
	Hwnd           uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Pt             Point
	Private        uint32
}
type WndClass struct {
	Size, Style                        uint32
	Proc                               uintptr
	ClsExtra, WndExtra                 int32
	Instance, Icon, Cursor, Background uintptr
	Menu, Class                        *uint16
	IconSmall                          uintptr
}
type Paint struct {
	DC                 uintptr
	Erase              int32
	Rect               Rect
	Restore, IncUpdate int32
	Reserved           [32]byte
}
type MinMaxInfo struct{ Reserved, MaxSize, MaxPosition, MinTrackSize, MaxTrackSize Point }
type MonitorInfo struct {
	Size          uint32
	Monitor, Work Rect
	Flags         uint32
}
type Size struct{ CX, CY int32 }
type OpenFile struct {
	Size                         uint32
	Owner, Instance              uintptr
	Filter, CustomFilter         *uint16
	MaxCustomFilter, FilterIndex uint32
	File                         *uint16
	MaxFile                      uint32
	FileTitle                    *uint16
	MaxFileTitle                 uint32
	InitialDir, Title            *uint16
	Flags                        uint32
	FileOffset, FileExtension    uint16
	DefExt                       *uint16
	CustData, Hook               uintptr
	TemplateName                 *uint16
	Reserved                     uintptr
	Reserved2, FlagsEx           uint32
}
type NotifyIcon struct {
	Size                uint32
	Hwnd                uintptr
	ID, Flags, Callback uint32
	Icon                uintptr
	Tip                 [128]uint16
	State, StateMask    uint32
	Info                [256]uint16
	Version             uint32
	InfoTitle           [64]uint16
	InfoFlags           uint32
	Guid                [16]byte
	BalloonIcon         uintptr
}

const (
	WM_CREATE           = 0x1
	WM_DESTROY          = 0x2
	WM_MOVE             = 0x3
	WM_SIZE             = 0x5
	WM_ACTIVATE         = 0x6
	WM_SETFOCUS         = 0x7
	WM_KILLFOCUS        = 0x8
	WM_PAINT            = 0xf
	WM_CLOSE            = 0x10
	WM_QUERYENDSESSION  = 0x11
	WM_ENDSESSION       = 0x16
	WM_ERASEBKGND       = 0x14
	WM_GETMINMAXINFO    = 0x24
	WM_SETCURSOR        = 0x20
	WM_DISPLAYCHANGE    = 0x7e
	WM_NCCALCSIZE       = 0x83
	WM_NCHITTEST        = 0x84
	WM_KEYDOWN          = 0x100
	WM_SYSKEYDOWN       = 0x104
	WM_CHAR             = 0x102
	WM_COMMAND          = 0x111
	WM_TIMER            = 0x113
	WM_MOUSEMOVE        = 0x200
	WM_LBUTTONDOWN      = 0x201
	WM_LBUTTONUP        = 0x202
	WM_LBUTTONDBLCLK    = 0x203
	WM_RBUTTONUP        = 0x205
	WM_MOUSEWHEEL       = 0x20a
	WM_HOTKEY           = 0x312
	WM_DROPFILES        = 0x233
	WM_ENTERSIZEMOVE    = 0x231
	WM_EXITSIZEMOVE     = 0x232
	WM_DPICHANGED       = 0x2e0
	WM_APP              = 0x8000
	WS_POPUP            = 0x80000000
	WS_VISIBLE          = 0x10000000
	WS_CHILD            = 0x40000000
	WS_TABSTOP          = 0x10000
	WS_BORDER           = 0x800000
	WS_THICKFRAME       = 0x40000
	WS_EX_TOPMOST       = 0x8
	WS_EX_TOOLWINDOW    = 0x80
	WS_EX_LAYERED       = 0x80000
	WS_EX_NOACTIVATE    = 0x8000000
	WS_EX_CONTROLPARENT = 0x10000
	WS_EX_CLIENTEDGE    = 0x200
	SW_HIDE             = 0
	SW_SHOWNORMAL       = 1
	SW_SHOW             = 5
	SW_SHOWNOACTIVATE   = 4
	SWP_NOSIZE          = 1
	SWP_NOMOVE          = 2
	SWP_NOZORDER        = 4
	SWP_NOACTIVATE      = 0x10
	SWP_FRAMECHANGED    = 0x20
	MOD_ALT             = 1
	MOD_CONTROL         = 2
	MOD_SHIFT           = 4
	MOD_WIN             = 8
	MOD_NOREPEAT        = 0x4000
	VK_LEFT             = 0x25
	VK_UP               = 0x26
	VK_RIGHT            = 0x27
	VK_DOWN             = 0x28
	VK_ESCAPE           = 0x1b
	VK_SPACE            = 0x20
	VK_RETURN           = 0xd
	VK_TAB              = 9
	DT_LEFT             = 0
	DT_CENTER           = 1
	DT_RIGHT            = 2
	DT_VCENTER          = 4
	DT_WORDBREAK        = 0x10
	DT_SINGLELINE       = 0x20
	DT_NOPREFIX         = 0x800
	DT_END_ELLIPSIS     = 0x8000
)

var Topmost = ^uintptr(0)

func Client(hwnd uintptr) Rect {
	var r Rect
	U("GetClientRect", hwnd, uintptr(unsafe.Pointer(&r)))
	return r
}
func Bounds(hwnd uintptr) Rect {
	var r Rect
	U("GetWindowRect", hwnd, uintptr(unsafe.Pointer(&r)))
	return r
}
func SetText(hwnd uintptr, s string) { U("SetWindowTextW", hwnd, uintptr(unsafe.Pointer(Str(s)))) }
func Text(hwnd uintptr) string {
	n := int(U("GetWindowTextLengthW", hwnd)) + 1
	b := make([]uint16, n)
	U("GetWindowTextW", hwnd, uintptr(unsafe.Pointer(&b[0])), uintptr(n))
	return syscall.UTF16ToString(b)
}
func DrawText(dc uintptr, s string, r Rect, flags uint32) {
	u := UTF16(s)
	if len(u) == 0 {
		return
	}
	U("DrawTextW", dc, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)), uintptr(unsafe.Pointer(&r)), uintptr(flags|DT_NOPREFIX))
}
func Fill(dc uintptr, r Rect, color uintptr) {
	b := G("CreateSolidBrush", color)
	U("FillRect", dc, uintptr(unsafe.Pointer(&r)), b)
	G("DeleteObject", b)
}
func Message(hwnd uintptr, text, title string, flags uintptr) int {
	return int(U("MessageBoxW", hwnd, uintptr(unsafe.Pointer(Str(text))), uintptr(unsafe.Pointer(Str(title))), flags))
}
func WorkArea(hwnd uintptr) Rect {
	m := U("MonitorFromWindow", hwnd, 2)
	mi := MonitorInfo{Size: uint32(unsafe.Sizeof(MonitorInfo{}))}
	U("GetMonitorInfoW", m, uintptr(unsafe.Pointer(&mi)))
	if mi.Work.Width() == 0 {
		mi.Work = Rect{0, 0, 1920, 1080}
	}
	return mi.Work
}
