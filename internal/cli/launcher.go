package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/zhaodengfeng/dtsw/internal/config"
	"github.com/zhaodengfeng/dtsw/internal/install"
)

type launcherState struct {
	ConfigPath string
	Config     config.Config
	Ready      bool
	Problem    error
}

func runLauncher(stdout, stderr io.Writer) int {
	input, cleanup, err := openSetupInput()
	if err != nil {
		printUsage(stdout)
		return 0
	}
	defer cleanup()

	reader := bufio.NewReader(input)
	for {
		state := findLauncherState(defaultLauncherConfigPaths())
		renderLauncher(stdout, state)

		choice, err := panelPromptDefault(reader, stdout, "请选择操作", launcherDefaultChoice(state))
		if err != nil {
			fmt.Fprintf(stderr, "读取输入失败: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "")

		switch {
		case state.Ready:
			switch strings.TrimSpace(choice) {
			case "0", "q", "quit", "exit":
				fmt.Fprintln(stdout, "已退出 DTSW。")
				return 0
			case "1":
				return runPanelWithInput(state.ConfigPath, reader, stdout, stderr)
			case "2":
				if os.Geteuid() != 0 {
					fmt.Fprintln(stdout, "此操作需要 root 权限，请以 root 身份重启 DTSW 后重试。")
					waitForEnter(reader, stdout)
					continue
				}
				if err := install.Execute(context.Background(), state.Config, install.Options{Stdout: stdout, Stderr: stderr}); err != nil {
					fmt.Fprintf(stderr, "安装或修复失败: %v\n", err)
					waitForEnter(reader, stdout)
					continue
				}
				fmt.Fprintln(stdout, "服务器安装或修复已完成。")
				fmt.Fprintln(stdout, "")
				printClientConfiguration(stdout, state.Config)
				fmt.Fprintln(stdout, "")
				fmt.Fprintln(stdout, "正在打开 DTSW 管理面板...")
				return runPanelWithInput(state.ConfigPath, reader, stdout, stderr)
			case "3":
				printClientConfiguration(stdout, state.Config)
				waitForEnter(reader, stdout)
			case "4":
				return runSetup(stdout, stderr)
			default:
				fmt.Fprintf(stderr, "无效的选项 %q\n", choice)
				waitForEnter(reader, stdout)
			}
		default:
			switch strings.TrimSpace(choice) {
			case "0", "q", "quit", "exit":
				fmt.Fprintln(stdout, "已退出 DTSW。")
				return 0
			case "1":
				return runSetup(stdout, stderr)
			default:
				fmt.Fprintf(stderr, "无效的选项 %q\n", choice)
				waitForEnter(reader, stdout)
			}
		}
	}
}

func renderLauncher(stdout io.Writer, state launcherState) {
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "╔══════════════════════════════════════╗")
	fmt.Fprintln(stdout, "║           DTSW 交互菜单              ║")
	fmt.Fprintln(stdout, "╚══════════════════════════════════════╝")
	fmt.Fprintln(stdout, "")

	if state.Ready {
		fmt.Fprintf(stdout, "  配置文件:           %s\n", state.ConfigPath)
		fmt.Fprintf(stdout, "  域名:              %s\n", state.Config.Server.Domain)
		fmt.Fprintf(stdout, "  用户数:            %d\n", len(state.Config.Users))
		fmt.Fprintln(stdout, "")
		fmt.Fprintln(stdout, "  1) 打开管理面板")
		fmt.Fprintln(stdout, "  2) 使用已保存配置安装或修复")
		fmt.Fprintln(stdout, "  3) 查看客户端配置")
		fmt.Fprintln(stdout, "  4) 重新运行引导安装")
		fmt.Fprintln(stdout, "  0) 退出")
		fmt.Fprintln(stdout, "")
		return
	}

	if state.Problem != nil {
		fmt.Fprintf(stdout, "  发现配置文件 %s，但存在问题:\n", state.ConfigPath)
		fmt.Fprintf(stdout, "  %v\n", state.Problem)
	} else {
		fmt.Fprintln(stdout, "  尚未找到 DTSW 配置文件。")
	}
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "  1) 开始引导安装")
	fmt.Fprintln(stdout, "  0) 退出")
	fmt.Fprintln(stdout, "")
}

func launcherDefaultChoice(state launcherState) string {
	return "1"
}

func defaultLauncherConfigPaths() []string {
	paths := []string{config.DefaultPaths().DTSWConfigFile}
	if local, err := filepath.Abs("./dtsw.config.json"); err == nil {
		if local != paths[0] {
			paths = append(paths, local)
		}
	} else {
		paths = append(paths, "./dtsw.config.json")
	}
	return paths
}

func findLauncherState(paths []string) launcherState {
	for _, path := range paths {
		cfg, err := config.Load(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return launcherState{ConfigPath: path, Problem: err}
		}
		if err := cfg.Validate(); err != nil {
			return launcherState{ConfigPath: path, Problem: err}
		}
		return launcherState{ConfigPath: path, Config: cfg, Ready: true}
	}
	return launcherState{}
}
