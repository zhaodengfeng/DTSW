# DTSW

DTSW 全称 `Does Trojan Still Work?`，是一个基于 Xray 的 Trojan 代理服务管理工具，灵感来源于 `easytrojan`，但只支持用户自有域名，不依赖免费共享域名。

当前实现使用：

- Go 控制平面（`dtsw`）
- Xray 作为 Trojan 运行时
- `acme.sh` 作为 ACME 客户端，支持 `Let's Encrypt` 和 `ZeroSSL`
- Caddy 静态网站作为回落服务
- `systemd` 单元管理运行时、回落服务和自动续签

## 一键安装

从最新 Release 下载安装脚本，自动校验二进制完整性并启动交互式安装向导：

```bash
curl -fsSL https://github.com/zhaodengfeng/DTSW/releases/latest/download/install.sh | bash
```

安装完成后，DTSW 会：

- 保存配置文件
- 以 root 运行时自动安装或修复服务
- 复用已有的有效证书，不重复申请
- 打印客户端连接信息
- 自动打开管理面板，无需手动输入命令

## 交互流程

标准操作全程菜单驱动：

1. 运行安装脚本
2. 回答引导安装向导的问题
3. DTSW 自动完成服务器安装
4. 使用管理面板查看客户端配置、运行状态、安装修复、升级 Xray、续签证书、管理多用户、查看流量统计或卸载

直接运行 `dtsw`（不带参数）会打开交互式启动菜单。若已有保存的配置，菜单提供：

- 打开管理面板
- 使用已保存配置安装或修复
- 查看客户端配置
- 重新运行引导安装

## 已实现功能

- 交互式安装向导，以 root 运行时自动完成安装
- 零参数启动的交互式启动菜单
- 管理面板：一键升级 Xray、修复、菜单化用户管理、菜单化卸载
- **用户流量统计**：查看每个用户的总流量和当月流量，数据持久化，自动处理重启后计数器归零
- 初始化、校验和渲染 DTSW/Xray 配置
- 生成运行时、回落、续签 `systemd` 单元
- 安装 DTSW、锁定版本的 `acme.sh`、Xray、Caddy，以及配置文件、回落网站内容和服务
- 重装时复用已有有效证书
- 安装完成后打印客户端连接信息
- 通过 `Let's Encrypt` 或 `ZeroSSL` 申请和续签证书
- 通过 `status` 和 `doctor` 检查运行状态
- 通过 `list`、`add`、`del`、`url` 管理 Trojan 用户
- 卸载受管服务和生成的文件

## 使用前提

- 实际安装目标为带 `systemd` 的 Linux 系统
- 证书自动化使用锁定版本的 `acme.sh` 脚本 + DTSW 管理的续签定时器
- HTTP-01 验证需要 TCP `80` 端口可访问
- DNS-01 验证需要在 `/etc/dtsw/acme.env` 中配置服务商凭证
- 首次安装时会自动获取最新稳定版 Xray 并写入配置；获取失败时回落到内置版本 `v26.1.13`
- 新安装默认在 `127.0.0.1:8080` 启动 Caddy 静态回落网站
- 默认 Caddy 版本锁定为 `v2.10.2`
- 默认 `acme.sh` 版本锁定为 `3.1.2`

## 高级命令

交互菜单是主要操作路径，以下命令行操作仍可用于自动化和调试：

```bash
dtsw
dtsw status --config /etc/dtsw/config.json
dtsw doctor --config /etc/dtsw/config.json
sudo dtsw runtime upgrade --config /etc/dtsw/config.json --latest
```

## 设计决策

- 必须使用自有域名。不支持 IP 地址、免费通配符域名或公共共享域名。
- 证书生命周期独立于运行时，由 DTSW 统一管理 CA 切换、诊断和重载行为，不嵌入代理引擎。
- 新安装使用 DTSW 管理的 Caddy 静态网站处理回落流量，旧版内置回落页面作为兼容模式保留在配置中。
- 选择 Xray 作为第一个运行时后端，是 Trojan 场景下的保守默认值，代码预留了接入其他运行时适配器的空间。
- 含密码的配置文件以 `0600` 权限写入，保障安全。
