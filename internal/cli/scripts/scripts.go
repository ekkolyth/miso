package scripts

import (
	"fmt"
	"os"
	"sort"

	"github.com/charmbracelet/log"

	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/ui"
)

// miso scripts
func List(cfg config.Config, styles ui.Styles, logger *log.Logger) error {
	if len(cfg.Scripts) == 0 {
		logger.Info("no scripts defined in miso.json")
		return nil
	}
	scriptNames := make([]string, 0, len(cfg.Scripts))
	for name := range cfg.Scripts {
		scriptNames = append(scriptNames, name)
	}
	sort.Strings(scriptNames)
	logger.Info(styles.Heading.Render("available scripts"))
	for _, name := range scriptNames {
		fmt.Fprintf(os.Stdout, "  %s: %s\n", name, cfg.Scripts[name])
	}
	return nil
}
