package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"

	"floatreader/internal/reader"
	w "floatreader/internal/win32"
)

const (
	initialBookChunk = 64 * 1024
	maximumBookChunk = 8 * 1024 * 1024
	maximumBookSize  = int64(512 * 1024 * 1024)
)

type bookEncoding uint8

const (
	bookUTF8 bookEncoding = iota
	bookUTF16LE
	bookUTF16BE
	bookGB18030
)

type bookSource struct {
	file          *os.File
	size          int64
	contentStart  int64
	encoding      bookEncoding
	encodingLabel string
	peakRead      int
	readCalls     int
}

type decodedWindow struct {
	text     []rune
	ends     []int64
	consumed int64
	eof      bool
}

type loadedPage struct {
	Text       []rune
	Layout     reader.Page
	Start, End int64
	EOF        bool
}

func openBookSource(path string) (*bookSource, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("选中的是文件夹，请选择 TXT 文件")
	}
	if info.Size() > maximumBookSize {
		return nil, fmt.Errorf("文件超过 512 MB，请分卷导入")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	b := &bookSource{file: f, size: info.Size(), encoding: bookUTF8, encodingLabel: "UTF-8"}
	head := make([]byte, min(3, int(info.Size())))
	_, _ = f.ReadAt(head, 0)
	switch {
	case len(head) >= 2 && head[0] == 0xff && head[1] == 0xfe:
		b.encoding, b.encodingLabel, b.contentStart = bookUTF16LE, "UTF-16", 2
	case len(head) >= 2 && head[0] == 0xfe && head[1] == 0xff:
		b.encoding, b.encodingLabel, b.contentStart = bookUTF16BE, "UTF-16", 2
	case len(head) >= 3 && head[0] == 0xef && head[1] == 0xbb && head[2] == 0xbf:
		b.contentStart = 3
	case !validUTF8Stream(f, 0, info.Size()):
		b.encoding, b.encodingLabel = bookGB18030, "GB18030 / GBK"
	}
	return b, nil
}

func validUTF8Stream(f *os.File, start, length int64) bool {
	r := bufio.NewReaderSize(io.NewSectionReader(f, start, length), initialBookChunk)
	for {
		ru, size, err := r.ReadRune()
		if err == io.EOF {
			return true
		}
		if err != nil || ru == utf8.RuneError && size == 1 {
			return false
		}
	}
}

func (b *bookSource) Close() {
	if b != nil && b.file != nil {
		_ = b.file.Close()
		b.file = nil
	}
}

func (b *bookSource) progress(offset int64) int {
	length := b.size - b.contentStart
	if length <= 0 {
		return 100
	}
	return clamp(int((offset-b.contentStart)*100/length), 0, 100)
}

func (b *bookSource) loadPage(start int64, width, rows, limit int, fit reader.Fit) (loadedPage, error) {
	start = max(b.contentStart, min(start, b.size))
	chunkSize := initialBookChunk
	for {
		window, err := b.readWindow(start, chunkSize)
		if err != nil {
			return loadedPage{}, err
		}
		page, full := reader.PaginatePage(window.text, width, rows, limit, fit)
		if full || window.eof {
			end := start
			if page.End > 0 {
				end = window.ends[page.End-1]
			}
			eof := window.eof && page.End == len(window.text)
			if eof {
				end = b.size
			}
			page.Start = 0
			return loadedPage{Text: append([]rune(nil), window.text[:page.End]...), Layout: page, Start: start, End: end, EOF: eof}, nil
		}
		if chunkSize >= maximumBookChunk {
			return loadedPage{}, fmt.Errorf("当前窗口一页需要读取过多文字，请缩小窗口或设置每页字数")
		}
		chunkSize = min(maximumBookChunk, chunkSize*2)
	}
}

func (b *bookSource) readWindow(start int64, byteLimit int) (decodedWindow, error) {
	runes, ends, consumed, eof, err := b.decodeRaw(start, byteLimit)
	if err != nil {
		return decodedWindow{}, err
	}
	text := make([]rune, 0, len(runes))
	normalizedEnds := make([]int64, 0, len(runes))
	for i := 0; i < len(runes); i++ {
		ru := runes[i]
		switch ru {
		case 0:
			continue
		case '\r':
			if i+1 == len(runes) && !eof {
				consumed = runeStart(start, ends, i)
				i = len(runes)
				continue
			}
			if i+1 < len(runes) && runes[i+1] == '\n' {
				i++
			}
			text = append(text, '\n')
			normalizedEnds = append(normalizedEnds, ends[i])
		case '\t':
			text = append(text, ' ')
			normalizedEnds = append(normalizedEnds, ends[i])
		default:
			text = append(text, ru)
			normalizedEnds = append(normalizedEnds, ends[i])
		}
	}
	return decodedWindow{text: text, ends: normalizedEnds, consumed: consumed, eof: eof}, nil
}

func runeStart(start int64, ends []int64, i int) int64 {
	if i == 0 {
		return start
	}
	return ends[i-1]
}

func (b *bookSource) decodeRaw(start int64, byteLimit int) ([]rune, []int64, int64, bool, error) {
	remaining := b.size - start
	if remaining <= 0 {
		return nil, nil, start, true, nil
	}
	n := min(int64(byteLimit), remaining)
	raw := make([]byte, int(n))
	read, err := b.file.ReadAt(raw, start)
	if err != nil && err != io.EOF {
		return nil, nil, start, false, err
	}
	raw = raw[:read]
	b.readCalls++
	if read > b.peakRead {
		b.peakRead = read
	}
	eof := start+int64(read) >= b.size
	switch b.encoding {
	case bookUTF16LE, bookUTF16BE:
		return b.decodeUTF16(raw, start, eof)
	case bookGB18030:
		return b.decodeGB18030(raw, start, eof)
	default:
		return decodeUTF8(raw, start, eof)
	}
}

func decodeUTF8(raw []byte, start int64, eof bool) ([]rune, []int64, int64, bool, error) {
	runes := make([]rune, 0, len(raw)/2)
	ends := make([]int64, 0, cap(runes))
	i := 0
	for i < len(raw) {
		if !utf8.FullRune(raw[i:]) && !eof {
			break
		}
		ru, size := utf8.DecodeRune(raw[i:])
		if ru == utf8.RuneError && size == 1 {
			return nil, nil, start + int64(i), false, fmt.Errorf("TXT 中有无效的 UTF-8 字节")
		}
		i += size
		runes = append(runes, ru)
		ends = append(ends, start+int64(i))
	}
	return runes, ends, start + int64(i), eof && i == len(raw), nil
}

func (b *bookSource) decodeUTF16(raw []byte, start int64, eof bool) ([]rune, []int64, int64, bool, error) {
	var order binary.ByteOrder = binary.LittleEndian
	if b.encoding == bookUTF16BE {
		order = binary.BigEndian
	}
	runes := make([]rune, 0, len(raw)/2)
	ends := make([]int64, 0, cap(runes))
	i := 0
	for i+1 < len(raw) {
		u := order.Uint16(raw[i : i+2])
		if u >= 0xd800 && u <= 0xdbff {
			if i+3 >= len(raw) {
				if eof {
					return nil, nil, start + int64(i), false, fmt.Errorf("UTF-16 文件末尾不完整")
				}
				break
			}
			v := order.Uint16(raw[i+2 : i+4])
			if v < 0xdc00 || v > 0xdfff {
				return nil, nil, start + int64(i), false, fmt.Errorf("UTF-16 文件中有无效字符")
			}
			runes = append(runes, utf16.DecodeRune(rune(u), rune(v)))
			i += 4
		} else if u >= 0xdc00 && u <= 0xdfff {
			return nil, nil, start + int64(i), false, fmt.Errorf("UTF-16 文件中有无效字符")
		} else {
			runes = append(runes, rune(u))
			i += 2
		}
		ends = append(ends, start+int64(i))
	}
	if eof && i != len(raw) {
		return nil, nil, start + int64(i), false, fmt.Errorf("UTF-16 文件长度异常，可能未下载完整")
	}
	return runes, ends, start + int64(i), eof && i == len(raw), nil
}

func (b *bookSource) decodeGB18030(raw []byte, start int64, eof bool) ([]rune, []int64, int64, bool, error) {
	ends := make([]int64, 0, len(raw)/2)
	i := 0
	for i < len(raw) {
		size := 1
		if raw[i] >= 0x80 {
			if i+1 >= len(raw) {
				if eof {
					return nil, nil, start + int64(i), false, fmt.Errorf("GB18030 文件末尾不完整")
				}
				break
			}
			size = 2
			if raw[i+1] >= '0' && raw[i+1] <= '9' {
				size = 4
			}
			if i+size > len(raw) {
				if eof {
					return nil, nil, start + int64(i), false, fmt.Errorf("GB18030 文件末尾不完整")
				}
				break
			}
		}
		i += size
		ends = append(ends, start+int64(i))
	}
	if i == 0 {
		return nil, nil, start, eof, nil
	}
	var u []uint16
	for _, codePage := range []uintptr{54936, 936} {
		uCount := w.K("MultiByteToWideChar", codePage, 8, uintptr(unsafe.Pointer(&raw[0])), uintptr(i), 0, 0)
		if uCount == 0 {
			continue
		}
		u = make([]uint16, uCount)
		if w.K("MultiByteToWideChar", codePage, 8, uintptr(unsafe.Pointer(&raw[0])), uintptr(i), uintptr(unsafe.Pointer(&u[0])), uCount) != 0 {
			break
		}
		u = nil
	}
	if len(u) == 0 {
		return nil, nil, start, false, fmt.Errorf("TXT 中有无效的 GB18030 / GBK 字节")
	}
	runes := utf16.Decode(u)
	if len(runes) != len(ends) {
		return nil, nil, start, false, fmt.Errorf("GB18030 / GBK 字符边界异常")
	}
	return runes, ends, start + int64(i), eof && i == len(raw), nil
}

// byteOffsetForRune migrates reading positions written by versions before
// chunked loading. It scans fixed-size blocks and never retains the whole book.
func (b *bookSource) byteOffsetForRune(target int) (int64, error) {
	if target <= 0 {
		return b.contentStart, nil
	}
	pos, count := b.contentStart, 0
	for pos < b.size {
		runes, ends, consumed, eof, err := b.decodeRaw(pos, initialBookChunk)
		if err != nil {
			return pos, err
		}
		for i := 0; i < len(runes); i++ {
			ru := runes[i]
			if ru == 0 {
				continue
			}
			if ru == '\r' && i+1 < len(runes) && runes[i+1] == '\n' {
				i++
			}
			step := 1
			if ru == '\t' {
				step = 4
			}
			if count+step >= target {
				return ends[i], nil
			}
			count += step
		}
		if consumed <= pos {
			break
		}
		pos = consumed
		if eof {
			break
		}
	}
	return min(pos, b.size), nil
}
