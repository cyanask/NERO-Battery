# USB 外设输出功率

1.2.0 优先读取 AppleSMC 的 USB-C 输出电压 `DnJV` 和输出电流 `DnJI`，以两者相乘得到功率，并明确标注“电压 × 电流换算”。它是传感器读数的换算，不是独立功率计校准，也不用 PD 协商额度、申报电流或系统功率差替代输出。

## 端口与读取校验

先读取 `AppleHPMInterface` 的 USB-C 接口，再以通道 `DnUI` 与其上级 `AppleHPMDevice` 自有 UUID 精确匹配。SMC 的 D 编号不直接当作物理端口号。匹配缺失、多义或重复时保持未知；MagSafe 不进入 USB-C 合计。UUID 只在内存用于匹配，不进入界面、诊断 JSON 或本地历史。

前端独立读端只发送命令 9（字段信息）和 5（字段数据），没有写入操作。它校验 IOKit 返回码、结构长度、SMC 的 `result` 和 `status`、字段类型、数据长度及有限数值范围。读取失败不转为零；部分端口不可用时，合计保持未知，仍展示有效端口。该校验覆盖新 USB 采集路径，本版没有修改后台、固定控制客户端或其共享 SMC 库。

键与端口匹配方法可对照 [WhatCable 的读取实现](https://github.com/darrylmorley/whatcable/blob/6dcd0c37a20358a8c7234aaa887a36f3f8d5316d/Sources/WhatCableDarwinBackend/Reading/SMCPowerReader.swift) 和 [输出通道定义](https://github.com/darrylmorley/whatcable/blob/6dcd0c37a20358a8c7234aaa887a36f3f8d5316d/Sources/WhatCableCore/Power/SMCPortPowerChannel.swift)。本版没有复制其源文件或新增运行依赖。

## 后备来源

所有 SMC 端口都不可用时，才读取 `AppleSmartBattery.PowerOutDetails` 的 `Watts`，除以 1000 转为 W，并在界面标为“系统报告，刷新可能较慢”。不同来源不混合合计。后备合计只覆盖系统报告的完整条目；条目缺失时保持未知，系统索引不猜测为设备名称。

这条系统报告路径已由官方 AlDente 1.40.1 镜像的只读静态分析确认，镜像 SHA256 为 `cbe3f47f1b08b9de5760caf46b4965a6ec80b1ff8b5f043a7e83b952b006ca23`。AlDente 的处理在 `Watts` 缺失或转换失败时还会读同一条目的 `PDPowermW`。现有公开实现和实机记录支持将后者视为 PD 协商额度，NERO 不采用该兜底。参见 [字段定义](https://github.com/darrylmorley/whatcable/blob/6dcd0c37a20358a8c7234aaa887a36f3f8d5316d/Sources/WhatCableCore/Power/PowerTelemetry.swift)。静态分析没有安装、运行或激活 AlDente，没有把其代码、二进制或图形资产加入 NERO。

## 本机验证与边界

在 M4 / Mac16,12、macOS 26.6.2 上，精确匹配到 D1、D2 为两个 USB-C 端口，D3 为 MagSafe。用户接入手机后，前期 30 秒只读核实得到约 5.110～5.121 V、1.399～1.722 A，换算约 7.16～8.81 W；192 次 SMC 设备返回码均通过校验。同一时段系统报告为 7.625 W，保持不变，SMC 读数则有自然变化。这些数值是当时记录，不是每次连接的承诺范围。

1.1.0 初次未报告的状态发生在未接入手机时，不能据此判断本机硬件不支持。当前已经验证有效的正值路径；其他机型、负载、长时刷新稳定性及绝对精度仍需独立验收，不以合成数据代替。

采集沿用现有总览刷新，只在总览窗口可见时执行，或由用户显式运行 `--usb-power` 做一次只读诊断。关闭窗口不新增采集，不申请额外权限，不上传数据；`sampled_at` 表示读取时间，不保证传感器内部采样时刻。后台充电策略和权限不变。本轮不进行放电测试。
