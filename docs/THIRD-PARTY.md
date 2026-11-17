# 上游、修改与第三方许可

NERO Battery 基于 [charlie0129/batt](https://github.com/charlie0129/batt) 定制。仓库许可证为 GNU GPL v2，原许可证文本保存在根目录 `LICENSE`。二进制分发时必须同时提供本版完整对应源码，包括 vendor、构建与安装脚本，并保留许可证和版权声明。源码包不包含 Apple SDK 或 Go 工具链；二者由构建者自行取得。

本次定制增加或修改了原生图形界面、本地状态与图表、充电安全流程、XPC 权限边界、固定控制客户端和可迁移发布工具。2026-11-16 的 1.0.0 完成可迁移交付，1.1.0 增加 USB 输出报告，2026-11-17 的 1.2.0 在前端独立实现 SMC 电压、电流只读采集及返回码校验；各文件的原有声明保留。SMC 键的含义和端口匹配方法参考公开资料及本机只读核实，没有引入新代码依赖。它不是 batt 或 AlDente 官方版本，不附带 AlDente 或 OpenDente 的二进制、许可证激活逻辑或图形资产。

| 依赖 | 固定版本 | 许可证文件 |
|---|---|---|
| charlie0129/gosmc | v0.0.0-20260712130932-24306d80e612 | vendor/github.com/charlie0129/gosmc/LICENSE，GPL v2 |
| pkg/errors | v0.9.1 | vendor/github.com/pkg/errors/LICENSE，BSD |
| robfig/cron/v3 | v3.0.1 | vendor/github.com/robfig/cron/v3/LICENSE，MIT |
| fatih/color | v1.15.0 | vendor/github.com/fatih/color/LICENSE.md，MIT |
| spf13/cobra | v1.10.2 | vendor/github.com/spf13/cobra/LICENSE.txt，Apache 2.0 |
| spf13/pflag | v1.0.9 | vendor/github.com/spf13/pflag/LICENSE，BSD |
| inconshreveable/mousetrap | v1.1.0 | vendor/github.com/inconshreveable/mousetrap/LICENSE，Apache 2.0 |
| mattn/go-colorable | v0.1.13 | vendor/github.com/mattn/go-colorable/LICENSE，MIT |
| mattn/go-isatty | v0.0.19 | vendor/github.com/mattn/go-isatty/LICENSE，MIT |
| golang.org/x/sys | v0.38.0 | vendor/golang.org/x/sys/LICENSE，BSD |
| golang.org/x/term | v0.37.0 | vendor/golang.org/x/term/LICENSE，BSD |

旧 batt CLI 仍保留在对应源码中，部分依赖仅被该 CLI 使用，不随 NERO 图形程序编译。正式二进制的实际依赖可用 `go version -m` 检查。上述许可证全文随源码保留，二进制归档另带许可文件及 Go 工具链的 BSD 许可证；不得只保留本表代替全文。

GNU GPL v2 正文及分发义务参见 [GNU 官方文本](https://www.gnu.org/licenses/old-licenses/gpl-2.0.en.html)。公开发布前应复核源代码、二进制与许可的对应关系；本次仅准备本地交付物，不执行发布。
