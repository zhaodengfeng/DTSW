package cli

import (
	"fmt"
	"io"

	"github.com/zhaodengfeng/dtsw/internal/config"
)

func completeInstallFlow(configPath string, cfg config.Config, stdout, stderr io.Writer) int {
	fmt.Fprintln(stdout, "")
	printClientConfiguration(stdout, cfg)

	input, cleanup, err := openSetupInput()
	if err != nil {
		fmt.Fprintln(stdout, "")
		fmt.Fprintln(stdout, "Installation finished. The interactive menu will open the next time DTSW starts in a terminal.")
		return 0
	}
	defer cleanup()

	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "Opening the DTSW management panel...")
	return runPanelWithInput(configPath, input, stdout, stderr)
}
