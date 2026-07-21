package tui

import "github.com/ekkolyth/miso/internal/config"

type Renderer int

const (
	RendererNone Renderer = iota
	RendererChrome
	RendererPlain
)

// chrome needs both a TTY and a non-off tui mode; otherwise orchestration runs
// plain. RendererNone is never returned here — callers gate on repo mode first.
func SelectRenderer(cfg config.Config, hasTTY bool) Renderer {
	if hasTTY && cfg.TuiMode != "off" {
		return RendererChrome
	}
	return RendererPlain
}
