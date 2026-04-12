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
		fmt.Fprintln(stdout, "安装已完成。下次在终端启动 DTSW 时将自动打开交互菜单。")
		return 0
	}
	defer cleanup()

	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "正在打开 DTSW 管理面板...")
	return runPanelWithInput(configPath, input, stdout, stderr)
}
