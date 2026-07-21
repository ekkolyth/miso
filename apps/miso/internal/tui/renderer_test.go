package tui

import (
	"testing"

	"github.com/ekkolyth/miso/internal/config"
)

func TestSelectRenderer(t *testing.T) {
	tabbed := config.Config{TuiMode: "tabbed"}
	off := config.Config{TuiMode: "off"}

	cases := []struct {
		name   string
		cfg    config.Config
		hasTTY bool
		want   Renderer
	}{
		{"tty + tabbed → chrome", tabbed, true, RendererChrome},
		{"no tty + tabbed → plain", tabbed, false, RendererPlain},
		{"tty + off → plain", off, true, RendererPlain},
		{"no tty + off → plain", off, false, RendererPlain},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SelectRenderer(tc.cfg, tc.hasTTY); got != tc.want {
				t.Errorf("SelectRenderer = %v, want %v", got, tc.want)
			}
		})
	}
}
