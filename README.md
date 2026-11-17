# NERO Battery Management

适用于苹果芯片 MacBook 的本地电池管理软件，基于 [batt](https://github.com/charlie0129/batt) 定制，采用原生 macOS 界面。1.2.0 是面向第三方测试的正式优化构建，尚不是经过多机型验收的稳定版。

## 适用范围

- 仅提供 ARM64 版本。需要 Apple Silicon、内置电池、macOS 14 或更高版本。
- 安装时必须通过 SMC 只读预检：支持 legacy 充电开关及适配器开关。只具备 firmware 限充接口的设备暂不接管。
- 目前仅在 M4 / Mac16,12、macOS 26.6.2 上有历史实机使用记录；1.2.0 的全新安装、卸载及其他机型仍需独立验收。系统版本下限不代表所有组合均已验证。
- 一个 Mac 仅授权一个本地账户，账户主目录须为 `/Users/用户名`。暂不支持多用户接管、网络主目录或 Intel Mac。

## USB 外设输出

总览优先显示 SMC 的 USB-C 输出电压、电流及两者相乘的功率，并标注“电压 × 电流换算”。端口通过控制器标识精确匹配，排除 MagSafe；读取核验 IOKit 与 SMC 返回码。部分端口缺失时合计保持未知，不补零；SMC 全部不可用时才退回系统输出报告，并说明来源。只在总览可见时采集，不增加关窗后的轮询。

本机接入手机后已读到非零值与自然变化，但未做独立功率计校准。不使用 PD 协商上限、申报电流或系统功率差代替输出，也不分摊扩展坞下设备功率。详见 [USB 数据口径](docs/USB-POWER.md)。

## 功能与边界

提供充电上下限、临时充满、手动放电、热保护、睡眠联动、定时任务、本地历史图表和菜单栏界面。控制客户端在空闲时退出。原生“快捷指令”动作尚不可用；校准、实际合盖及长时间定时行为仍有实机验收缺口。没有自动放电功能，也不声称完整复现 AlDente Pro。

数据仅保存在本机。应用没有遥测、上传、自动更新、网络监听或云账户功能。应用能耗分析由用户主动触发，处理本机进程名称和资源用量。详见 [隐私说明](docs/PRIVACY.md)。

## 安装与试用

1. 将二进制 ZIP 完整解压到本机目录，核对与同版源码对应的 SHA256。
2. 退出其他电池控制器并按其自身方式停止后台。安装程序遇到已运行的控制器会拒绝接管，不替你卸载其他软件。
3. 运行 `Check.command` 做普通账户预检。该步骤不接管充电。
4. 运行 `Install.command`，在 macOS 系统弹窗中完成一次管理员认证。成功后从 `~/Applications/NERO Battery.app` 打开应用。

没有 `.pkg`，无需安装 Go、Xcode 或其他开发工具。当前采用 ad-hoc 签名，没有 Developer ID 证书或公证。首次运行可能被 Gatekeeper 拦截；只能使用 macOS 针对该应用提供的正常确认方式。若系统无法提供确认，请停止试用；不要关闭 SIP、Gatekeeper，也不要批量移除隔离属性。参见 [Apple 官方说明](https://support.apple.com/en-us/102445)。

后台升级和卸载需要管理员认证；后台与固定客户端字节不变的前端更新可运行 `Update-GUI.command`，无需管理员认证。卸载运行 `Uninstall.command`：先恢复外接电源及充电开关，验证成功后删除运行组件，保留本地设置和历史。保留同版解压目录，便于卸载。

详见 [迁移与构建](docs/PORTABLE.md)、[试用验收](docs/TRIAL.md)、[安全边界](docs/SECURITY.md)、[第三方许可](docs/THIRD-PARTY.md)。仓库其余旧文档保留了上游 batt 的 CLI 用法，不能替代本版本安装说明。

## 源码与许可证

本项目按仓库的 GNU GPL v2 许可证分发；保留上游版权、许可证及修改说明。二进制必须与同版完整源码一起提供。源码包包含固定版本的 vendored 依赖、构建和安装脚本。使用 Apple Command Line Tools 与 Go 1.24.5 或更高版本在 Apple Silicon 上构建；本次构建使用 Go 1.27.4。

```sh
# 在源码仓库中导出干净源码；不包含已安装 App、个人设置或构建缓存。
sh packaging/release/export-source.sh /absolute/new-source
cd /absolute/new-source
sh packaging/release/build.sh /absolute/new-stage
```

不会创建发布证书、下载安装 Xcode、生成安装包或自动推送 GitHub。上游说明保存在 [UPSTREAM-README.md](docs/UPSTREAM-README.md)。
