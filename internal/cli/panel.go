package cli

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/zhaodengfeng/dtsw/internal/config"
	"github.com/zhaodengfeng/dtsw/internal/install"
	"github.com/zhaodengfeng/dtsw/internal/runtime/xray"
	"github.com/zhaodengfeng/dtsw/internal/stats"
	"github.com/zhaodengfeng/dtsw/internal/systemd"
	"github.com/zhaodengfeng/dtsw/internal/tlscfg"
)

type panelState struct {
	InstalledVersion string
	InstalledError   error
	LatestVersion    string
	LatestError      error
}

func runPanel(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("panel", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", config.DefaultPaths().DTSWConfigFile, "Path to config.json")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	return openPanelSession(*configPath, stdout, stderr)
}

func openPanelSession(configPath string, stdout, stderr io.Writer) int {
	input, cleanup, err := openSetupInput()
	if err != nil {
		fmt.Fprintf(stderr, "panel requires an interactive terminal: %v\n", err)
		return 1
	}
	defer cleanup()
	return runPanelWithInput(configPath, input, stdout, stderr)
}

func runPanelWithInput(configPath string, input io.Reader, stdout, stderr io.Writer) int {
	reader := bufio.NewReader(input)
	for {
		cfg, err := config.Load(configPath)
		if err != nil {
			fmt.Fprintf(stderr, "加载配置失败: %v\n", err)
			return 1
		}
		if err := cfg.Validate(); err != nil {
			fmt.Fprintf(stderr, "配置校验失败: %v\n", err)
			return 1
		}
		state := gatherPanelState(cfg)
		renderPanel(stdout, cfg, state)

		choice, err := panelPromptDefault(reader, stdout, "请选择操作", "1")
		if err != nil {
			fmt.Fprintf(stderr, "读取输入失败: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "")

		switch strings.TrimSpace(choice) {
		case "0", "q", "quit", "exit":
			fmt.Fprintln(stdout, "已退出 DTSW 管理面板。")
			return 0
		case "1":
			printClientConfiguration(stdout, cfg)
			waitForEnter(reader, stdout)
		case "2":
			printStatusReport(stdout, cfg)
			waitForEnter(reader, stdout)
		case "3":
			if err := ensureRootForPanelAction(); err != nil {
				fmt.Fprintln(stderr, err)
				waitForEnter(reader, stdout)
				continue
			}
			if err := install.Execute(context.Background(), cfg, install.Options{Stdout: stdout, Stderr: stderr}); err != nil {
				fmt.Fprintf(stderr, "安装或修复失败: %v\n", err)
			} else {
				fmt.Fprintln(stdout, "服务器安装或修复已完成。")
				fmt.Fprintln(stdout, "")
				printClientConfiguration(stdout, cfg)
			}
			waitForEnter(reader, stdout)
		case "4":
			if state.LatestError != nil {
				fmt.Fprintf(stderr, "获取最新 Xray 版本失败: %v\n", state.LatestError)
				waitForEnter(reader, stdout)
				continue
			}
			if err := ensureRootForPanelAction(); err != nil {
				fmt.Fprintln(stderr, err)
				waitForEnter(reader, stdout)
				continue
			}
			if err := upgradeRuntime(context.Background(), configPath, cfg, state.LatestVersion, install.Options{SkipIssue: true, Stdout: stdout, Stderr: stderr}); err != nil {
				fmt.Fprintf(stderr, "升级 Xray 失败: %v\n", err)
			} else {
				fmt.Fprintf(stdout, "Xray 已升级到 %s\n", state.LatestVersion)
			}
			waitForEnter(reader, stdout)
		case "5":
			if err := ensureRootForPanelAction(); err != nil {
				fmt.Fprintln(stderr, err)
				waitForEnter(reader, stdout)
				continue
			}
			if err := upgradeRuntime(context.Background(), configPath, cfg, cfg.Runtime.Version, install.Options{SkipIssue: true, Stdout: stdout, Stderr: stderr}); err != nil {
				fmt.Fprintf(stderr, "恢复 Xray 失败: %v\n", err)
			} else {
				fmt.Fprintf(stdout, "Xray 已恢复到配置版本 %s\n", cfg.Runtime.Version)
			}
			waitForEnter(reader, stdout)
		case "6":
			if err := ensureRootForPanelAction(); err != nil {
				fmt.Fprintln(stderr, err)
				waitForEnter(reader, stdout)
				continue
			}
			if err := tlscfg.Renew(context.Background(), cfg, tlscfg.Options{Stdout: stdout, Stderr: stderr}); err != nil {
				fmt.Fprintf(stderr, "续签证书失败: %v\n", err)
			} else {
				fmt.Fprintln(stdout, "TLS 证书续签完成。")
			}
			waitForEnter(reader, stdout)
		case "7":
			openUserPanel(configPath, cfg, reader, stdout, stderr)
		case "8":
			if err := ensureRootForPanelAction(); err != nil {
				fmt.Fprintln(stderr, err)
				waitForEnter(reader, stdout)
				continue
			}
			removed, err := uninstallFromPanel(cfg, reader, stdout, stderr)
			if err != nil {
				fmt.Fprintf(stderr, "卸载失败: %v\n", err)
				waitForEnter(reader, stdout)
				continue
			}
			if removed {
				fmt.Fprintln(stdout, "DTSW 已卸载完成。")
				return 0
			}
			waitForEnter(reader, stdout)
		default:
			fmt.Fprintf(stderr, "无效的选项 %q\n", choice)
			waitForEnter(reader, stdout)
		}
	}
}

func gatherPanelState(cfg config.Config) panelState {
	state := panelState{}
	if version, err := xray.CurrentVersion(cfg.Paths.XrayBinary); err == nil {
		state.InstalledVersion = version
	} else {
		state.InstalledError = err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if version, err := xray.LatestVersion(ctx); err == nil {
		state.LatestVersion = version
	} else {
		state.LatestError = err
	}
	return state
}

func renderPanel(stdout io.Writer, cfg config.Config, state panelState) {
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "╔══════════════════════════════════════╗")
	fmt.Fprintln(stdout, "║           DTSW 管理面板              ║")
	fmt.Fprintln(stdout, "╚══════════════════════════════════════╝")
	fmt.Fprintln(stdout, "")
	fmt.Fprintf(stdout, "  域名:              %s\n", cfg.Server.Domain)
	fmt.Fprintf(stdout, "  配置 Xray 版本:    %s\n", cfg.Runtime.Version)
	if state.InstalledError != nil {
		fmt.Fprintf(stdout, "  已安装 Xray 版本:  不可用 (%v)\n", state.InstalledError)
	} else {
		fmt.Fprintf(stdout, "  已安装 Xray 版本:  %s\n", state.InstalledVersion)
	}
	if state.LatestError != nil {
		fmt.Fprintf(stdout, "  最新 Xray 版本:    不可用 (%v)\n", state.LatestError)
	} else {
		fmt.Fprintf(stdout, "  最新 Xray 版本:    %s\n", state.LatestVersion)
	}
	fmt.Fprintf(stdout, "  用户数:            %d\n", len(cfg.Users))
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "  1) 查看客户端配置")
	fmt.Fprintln(stdout, "  2) 查看运行状态")
	fmt.Fprintln(stdout, "  3) 安装或修复服务器")
	fmt.Fprintln(stdout, "  4) 升级 Xray 到最新版")
	fmt.Fprintln(stdout, "  5) 恢复 Xray 到配置版本")
	fmt.Fprintln(stdout, "  6) 续签 TLS 证书")
	fmt.Fprintln(stdout, "  7) 用户与流量管理")
	fmt.Fprintln(stdout, "  8) 卸载 DTSW")
	fmt.Fprintln(stdout, "  0) 退出")
	fmt.Fprintln(stdout, "")
}

func openUserPanel(configPath string, cfg config.Config, reader *bufio.Reader, stdout, stderr io.Writer) {
	for {
		renderUserPanel(stdout, cfg)
		choice, err := panelPromptDefault(reader, stdout, "请选择操作", "1")
		if err != nil {
			fmt.Fprintf(stderr, "读取输入失败: %v\n", err)
			return
		}
		fmt.Fprintln(stdout, "")

		switch strings.TrimSpace(choice) {
		case "0", "q", "quit", "exit", "back":
			return
		case "1":
			printUserList(stdout, cfg)
			waitForEnter(reader, stdout)
		case "2":
			user, ok, err := selectUserFromConfig(reader, stdout, cfg, "查看哪个用户的配置")
			if err != nil {
				fmt.Fprintf(stderr, "选择用户失败: %v\n", err)
			} else if ok {
				printClientConfigurationForUser(stdout, cfg, user)
			}
			waitForEnter(reader, stdout)
		case "3":
			if err := ensureRootForPanelAction(); err != nil {
				fmt.Fprintln(stderr, err)
				waitForEnter(reader, stdout)
				continue
			}
			updated, changed, err := addUserFromPanel(configPath, cfg, reader, stdout, stderr)
			if err != nil {
				fmt.Fprintf(stderr, "添加用户失败: %v\n", err)
			} else if changed {
				cfg = updated
			}
			waitForEnter(reader, stdout)
		case "4":
			if err := ensureRootForPanelAction(); err != nil {
				fmt.Fprintln(stderr, err)
				waitForEnter(reader, stdout)
				continue
			}
			updated, changed, err := deleteUserFromPanel(configPath, cfg, reader, stdout, stderr)
			if err != nil {
				fmt.Fprintf(stderr, "删除用户失败: %v\n", err)
			} else if changed {
				cfg = updated
			}
			waitForEnter(reader, stdout)
		case "5":
			store, syncErr := stats.Sync(context.Background(), cfg.Paths.XrayBinary, cfg.Stats.APIListen, cfg.Paths.StatsFile)
			if syncErr != nil {
				fmt.Fprintf(stderr, "同步流量数据失败: %v\n", syncErr)
				store, _ = stats.LoadStore(cfg.Paths.StatsFile)
				if store == nil {
					store = &stats.Store{Users: make(map[string]*stats.UserStats)}
				}
				fmt.Fprintln(stderr, "警告: 无法连接 Xray，显示缓存数据。")
			}
			printAllUserStats(stdout, cfg, store)
			waitForEnter(reader, stdout)
		case "6":
			user, ok, err := selectUserFromConfig(reader, stdout, cfg, "查看哪个用户的流量")
			if err != nil {
				fmt.Fprintf(stderr, "选择用户失败: %v\n", err)
			} else if ok {
				store, syncErr := stats.Sync(context.Background(), cfg.Paths.XrayBinary, cfg.Stats.APIListen, cfg.Paths.StatsFile)
				if syncErr != nil {
					fmt.Fprintf(stderr, "同步流量数据失败: %v\n", syncErr)
					store, _ = stats.LoadStore(cfg.Paths.StatsFile)
					if store == nil {
						store = &stats.Store{Users: make(map[string]*stats.UserStats)}
					}
					fmt.Fprintln(stderr, "警告: 无法连接 Xray，显示缓存数据。")
				}
				printUserStats(stdout, user.Name, store)
			}
			waitForEnter(reader, stdout)
		default:
			fmt.Fprintf(stderr, "无效的选项 %q\n", choice)
			waitForEnter(reader, stdout)
		}
	}
}

func renderUserPanel(stdout io.Writer, cfg config.Config) {
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "╔══════════════════════════════════════╗")
	fmt.Fprintln(stdout, "║         用户与流量管理               ║")
	fmt.Fprintln(stdout, "╚══════════════════════════════════════╝")
	fmt.Fprintln(stdout, "")
	fmt.Fprintf(stdout, "  配置文件:           %s\n", cfg.Paths.DTSWConfigFile)
	fmt.Fprintf(stdout, "  用户数:             %d\n", len(cfg.Users))
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "  1) 用户列表")
	fmt.Fprintln(stdout, "  2) 查看用户客户端配置")
	fmt.Fprintln(stdout, "  3) 添加用户")
	fmt.Fprintln(stdout, "  4) 删除用户")
	fmt.Fprintln(stdout, "  5) 查看全部用户流量统计")
	fmt.Fprintln(stdout, "  6) 查看单个用户流量统计")
	fmt.Fprintln(stdout, "  0) 返回")
	fmt.Fprintln(stdout, "")
}

func addUserFromPanel(configPath string, cfg config.Config, reader *bufio.Reader, stdout, stderr io.Writer) (config.Config, bool, error) {
	defaultName := fmt.Sprintf("user%d", len(cfg.Users)+1)
	name, err := promptRequiredText(reader, stdout, "用户名", defaultName)
	if err != nil {
		return cfg, false, err
	}
	password, err := panelPromptDefault(reader, stdout, "Trojan 密码", generatePanelPassword())
	if err != nil {
		return cfg, false, err
	}
	if err := cfg.AddUser(name, password); err != nil {
		return cfg, false, err
	}
	if err := savePanelUserChanges(configPath, cfg, stdout, stderr); err != nil {
		return cfg, false, err
	}
	user, _ := cfg.User(name)
	fmt.Fprintf(stdout, "已添加用户 %s 并重载 Xray。\n\n", name)
	printClientConfigurationForUser(stdout, cfg, user)
	return cfg, true, nil
}

func deleteUserFromPanel(configPath string, cfg config.Config, reader *bufio.Reader, stdout, stderr io.Writer) (config.Config, bool, error) {
	user, ok, err := selectUserFromConfig(reader, stdout, cfg, "删除哪个用户")
	if err != nil || !ok {
		return cfg, false, err
	}
	confirm, err := panelPromptDefault(reader, stdout, fmt.Sprintf("确认删除用户 %s？(y/n)", user.Name), "n")
	if err != nil {
		return cfg, false, err
	}
	if !isAffirmative(confirm) {
		fmt.Fprintln(stdout, "已取消删除。")
		return cfg, false, nil
	}
	if err := cfg.DeleteUser(user.Name); err != nil {
		return cfg, false, err
	}
	if err := savePanelUserChanges(configPath, cfg, stdout, stderr); err != nil {
		return cfg, false, err
	}
	fmt.Fprintf(stdout, "已删除用户 %s 并重载 Xray。\n", user.Name)
	return cfg, true, nil
}

func savePanelUserChanges(configPath string, cfg config.Config, stdout, stderr io.Writer) error {
	if configPath != "" {
		cfg.Paths.DTSWConfigFile = configPath
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Paths.DTSWConfigFile), 0o755); err != nil {
		return err
	}
	if err := writeSourceAndRuntimeConfig(cfg.Paths.DTSWConfigFile, cfg, false, stdout, stderr); err != nil {
		return err
	}
	return systemd.Reload(context.Background(), systemd.CommandOptions{Stdout: stdout, Stderr: stderr}, cfg.Paths.RuntimeService)
}

func selectUserFromConfig(reader *bufio.Reader, stdout io.Writer, cfg config.Config, prompt string) (config.User, bool, error) {
	if len(cfg.Users) == 0 {
		fmt.Fprintln(stdout, "尚未配置任何用户。")
		return config.User{}, false, nil
	}
	fmt.Fprintln(stdout, "用户列表:")
	for i, user := range cfg.Users {
		fmt.Fprintf(stdout, "  %d) %s\n", i+1, user.Name)
	}
	selection, err := panelPromptDefault(reader, stdout, prompt, "1")
	if err != nil {
		return config.User{}, false, err
	}
	idx, err := strconv.Atoi(strings.TrimSpace(selection))
	if err != nil || idx < 1 || idx > len(cfg.Users) {
		return config.User{}, false, fmt.Errorf("无效的用户选择 %q", selection)
	}
	return cfg.Users[idx-1], true, nil
}

func promptRequiredText(reader *bufio.Reader, stdout io.Writer, label, defaultValue string) (string, error) {
	for {
		value, err := panelPromptDefault(reader, stdout, label, defaultValue)
		if err != nil {
			return "", err
		}
		value = strings.TrimSpace(value)
		if value != "" {
			return value, nil
		}
		fmt.Fprintln(stdout, "    ↳ 此项为必填。")
	}
}

func generatePanelPassword() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "change-me"
	}
	return hex.EncodeToString(b)
}

func isAffirmative(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	return value == "y" || value == "yes"
}

func uninstallFromPanel(cfg config.Config, reader *bufio.Reader, stdout, stderr io.Writer) (bool, error) {
	fmt.Fprintln(stdout, "卸载选项:")
	fmt.Fprintln(stdout, "  1) 仅移除服务，保留配置、证书和二进制文件")
	fmt.Fprintln(stdout, "  2) 移除服务和 DTSW 管理的数据，保留 DTSW 程序")
	fmt.Fprintln(stdout, "  3) 完全移除，包括 Xray、Caddy、数据和 DTSW 程序")
	fmt.Fprintln(stdout, "  0) 取消")

	choice, err := panelPromptDefault(reader, stdout, "选择卸载模式", "0")
	if err != nil {
		return false, err
	}
	mode := strings.TrimSpace(choice)
	if mode == "0" || strings.EqualFold(mode, "cancel") {
		fmt.Fprintln(stdout, "已取消卸载。")
		return false, nil
	}

	opts := install.RemoveOptions{
		Stdout: stdout,
		Stderr: stderr,
	}
	var confirmLabel string
	switch mode {
	case "1":
		confirmLabel = "确认仅移除 DTSW 服务？(y/n)"
	case "2":
		opts.PurgeData = true
		confirmLabel = "确认移除 DTSW 服务和托管数据？(y/n)"
	case "3":
		opts.PurgeData = true
		opts.PurgeXray = true
		opts.PurgeCaddy = true
		opts.RemoveDTSW = true
		confirmLabel = "确认完全移除 DTSW，包括数据和程序？(y/n)"
	default:
		return false, fmt.Errorf("无效的卸载选项 %q", choice)
	}

	confirm, err := panelPromptDefault(reader, stdout, confirmLabel, "n")
	if err != nil {
		return false, err
	}
	if !isAffirmative(confirm) {
		fmt.Fprintln(stdout, "已取消卸载。")
		return false, nil
	}

	if err := install.Remove(context.Background(), cfg, opts); err != nil {
		return false, err
	}
	return true, nil
}

func preferredPanelUser(cfg config.Config) (config.User, bool) {
	if user, ok := cfg.User("primary"); ok {
		return user, true
	}
	if len(cfg.Users) == 0 {
		return config.User{}, false
	}
	return cfg.Users[0], true
}

func waitForEnter(reader *bufio.Reader, stdout io.Writer) {
	fmt.Fprintln(stdout, "")
	fmt.Fprint(stdout, "按回车继续...")
	_, _ = reader.ReadString('\n')
}

func ensureRootForPanelAction() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("此操作需要 root 权限，请以 root 身份重启 DTSW 后重试")
	}
	return nil
}

func panelPromptDefault(reader *bufio.Reader, stdout io.Writer, label, defaultValue string) (string, error) {
	if defaultValue != "" {
		fmt.Fprintf(stdout, "  %s [%s]: ", label, defaultValue)
	} else {
		fmt.Fprintf(stdout, "  %s: ", label)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultValue, nil
	}
	return line, nil
}
