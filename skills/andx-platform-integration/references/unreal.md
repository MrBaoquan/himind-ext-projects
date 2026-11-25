# UE 缺口检查

> 状态：active ｜ 最后更新：2026-11-24 ｜ 负责人：AndX 维护者

本技能没有 UE 全能力实测基线。先找实际 uproject、uplugin、Build.cs、AndX/IODevice 插件和引擎版本。找不到实现时输出适配任务，不生成虚构包名或安装命令。

按请求链路检查：C++/Blueprint 公开能力、游戏线程回调、取消/对象销毁、IOUI Channel/RPC/Event、生命周期、契约版本与身份来源、本地动作映射。缺回调/运行时适配时不在 Blueprint 重写协议绕过。

Windows 验动态库/运行库，Android 验 ABI/JNI/插件打包/凭据设施。控制另验 TTL、幂等、授权/租约、安全停止与崩溃恢复。设备仍由单 Host 持有。

已有 SDK 可局部适配；新增 IODevice 路线按 IOUI-ANDX 架构先补缺失基础层。Editor 编译、打包、真实接口与设备结果分别记录，Windows 不传递给 Android，设计不当作实现证明。
