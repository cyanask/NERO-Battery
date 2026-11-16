# 迁移、构建与维护

## 目录与权限

| 组件 | 位置 | 权限 |
|---|---|---|
| 图形前端 | 当前授权用户的 `~/Applications/NERO Battery.app` | 用户所有，可独立更新 |
| 固定控制客户端 | `/Library/Application Support/NERO Battery/ControlClient/nero-control-client` | root 所有，用户不能改写 |
| 后台 | `/Library/PrivilegedHelperTools/io.nero.battery.helper` | root 所有，通过 launchd 启动 |
| 服务声明 | `/Library/LaunchDaemons/io.nero.battery.helper.plist` | root 所有 |
| XPC 策略 | `/Library/Application Support/NERO Battery/ipc-policy.json` | root 写入；固定 UID 与双方 CDHash |
| 设置、历史、恢复记录 | `/Library/Application Support/NERO Battery` | 留在本机；卸载默认保留 |

前端通过私有标准输入输出调用固定客户端，客户端通过 NSXPC 与后台通信。前端不作为 root 后台直接信任的调用方；客户端仍受固定身份、UID 及动作白名单校验。前端可以调用授权用户获准的充电动作，因此不要运行来源不明的前端。

安装程序从发起命令的普通用户识别账户，不写死用户名、UID 或开发目录。root 不通过用户可写的 Applications 路径复制文件；图形界面的放置、备份和删除均降为目标用户执行。

## 安装和升级

`Install.command` 同时用于首次安装和受保护组件升级。它完成发布清单检查、受保护目录检查、冲突控制器检查、受保护暂存区内的二次校验、SMC 只读能力探测，然后停止本应用旧后台，替换组件并按当前账户生成策略。设置与历史保留。新客户端须能通过 XPC 读取健康状态，才确认成功。

普通错误会触发文件、前端和原服务恢复。失败时保留 `/Library/PrivilegedHelperTools/.nero-setup-*` 下的恢复材料，错误消息会给出具体目录。该目录可能包含原配置的备份，勿公开上传。突然断电、进程被强制杀死等情况不能保证自动回退；先停止使用并保留现场，勿手工删除后台或恢复材料。

首次安装后手动打开 App。以后后台由 launchd 在开机时启动；前端由用户打开。暂不自动添加登录项。升级会关闭旧窗口，成功后手动重新打开即可。充电上下限与持久设置保留；临时充满、放电和保持唤醒等临时状态不会自动恢复。

若安装策略属于另一个 UID，程序拒绝转移；应先由原账户卸载。发现旧系统目录 App 的所有权异常时也会停止，不扩大权限或强行改所有者。

发起命令的普通用户先把选定的发布文件复制到临时目录，再交给管理员更新程序，完成后清理临时副本。后台更新不直接读取文稿或下载目录，不要求完全磁盘访问权限，也不修改系统隐私授权。

## 前端独立更新

`Update-GUI.command` 要求发布清单中的后台与控制客户端 CDHash 与本机受保护策略一致。满足时只替换当前用户的前端并做只读状态检查；失败则恢复前端。若提示后台不同，应使用 `Install.command` 完成该版本的受保护升级。前端独立更新并不授权修改固定客户端或后台策略。

## 卸载

保留解压目录，运行其中的 `Uninstall.command`。程序停止自有后台，使用已验证的同版工具恢复适配器和充电开关，读回确认后移除前端、客户端、后台、策略及服务声明。不能验证电源已交还 macOS 时停止卸载并保留恢复文件。设置与历史默认保留，未提供自动清空用户数据功能。

## 构建与复核

在 Apple Silicon Mac 上使用 Go 1.24.5+ 和 Apple Command Line Tools。无需完整 Xcode；不调用 `pkgbuild`、公证或证书申请。先运行 `export-source.sh`，再在导出的源码目录运行 `build.sh`。源码包括 vendor，构建使用 `-mod=vendor`，无需在线解析依赖；Go 工具链与 Apple SDK 不含在源码包内。

构建使用 `-p 2`、`-trimpath`、`-buildvcs=false` 和优化链接参数；输出 ARM64 二进制并做 ad-hoc 签名。源码摘要位于 `SOURCE-INFO.json`，二进制及文档逐文件摘要和代码身份位于 `release-manifest.json`。摘要不证明发布者身份，应通过你信任的分发渠道取得文件。

只读校验命令：

```sh
./bin/nero-setup --mode verify --release "$PWD"
./Check.command
```

构建成功和校验通过不能替代另一台 Mac 上的首次安装、重启及卸载验收。
