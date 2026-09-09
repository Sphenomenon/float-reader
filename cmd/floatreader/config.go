package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Hotkey struct {
	Key  uint32 `json:"key"`
	Mods uint32 `json:"modifiers"`
}
type BookMark struct {
	Path       string  `json:"path"`
	Offset     int     `json:"offset,omitempty"`
	ByteOffset int64   `json:"byte_offset,omitempty"`
	Page       int     `json:"page,omitempty"`
	Trail      []int64 `json:"trail,omitempty"`
}
type Config struct {
	Width      int        `json:"width"`
	Height     int        `json:"height"`
	X          int        `json:"x"`
	Y          int        `json:"y"`
	Positioned bool       `json:"positioned"`
	FontSize   int        `json:"font_size"`
	LineSpace  int        `json:"line_space_percent"`
	Opacity    int        `json:"opacity"`
	Theme      int        `json:"theme"`
	CharLimit  int        `json:"character_limit"`
	GlobalNav  bool       `json:"global_navigation"`
	Keys       [5]Hotkey  `json:"hotkeys"`
	Recent     []BookMark `json:"recent"`
}

func defaults() Config {
	return Config{Width: 460, Height: 580, FontSize: 20, LineSpace: 175, Opacity: 100,
		Keys: [5]Hotkey{{0x27, 0}, {0x25, 0}, {0x20, 3}, {'O', 2}, {0xbc, 2}}}
}

func loadConfig(path string) Config {
	c := defaults()
	data, err := os.ReadFile(path)
	if err == nil {
		if json.Unmarshal(data, &c) != nil {
			c = defaults()
		}
	}
	c.Width = clamp(c.Width, 320, 2000)
	c.Height = clamp(c.Height, 230, 2000)
	c.FontSize = clamp(c.FontSize, 14, 40)
	c.LineSpace = clamp(c.LineSpace, 120, 220)
	c.Opacity = clamp(c.Opacity, 0, 100)
	c.Theme = clamp(c.Theme, 0, 2)
	c.CharLimit = clamp(c.CharLimit, 0, 20000)
	for i, k := range c.Keys {
		if k.Key == 0 || k.Key > 0xfe || k.Mods > 15 {
			c.Keys[i] = defaults().Keys[i]
		}
	}
	if len(c.Recent) > 8 {
		c.Recent = c.Recent[:8]
	}
	return c
}

func writeConfig(path string, c Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(path+".tmp", data, 0600); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}

func clamp(v, lo, hi int) int { return min(hi, max(lo, v)) }
