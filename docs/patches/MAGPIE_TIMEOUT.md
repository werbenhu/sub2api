# Magpie 协议检测超时修复

## 原因

截图中的 OpenAI Chat Completions 和 Anthropic Messages 检测因客户端等待期限过短失败。线上接口实测返回 HTTP 200，分别耗时约 21.14 秒和 11.61 秒；日志中的 499 表示客户端先取消请求。Responses 实测也可用。按用户要求，不调整 Gemini。

安装版截图显示 12 秒；当前官方源码单次检测使用 20 秒、批量检测使用 12 秒，GUI 单次总期限为 25 秒。推理较慢时都可能误判协议不可用。

## 补丁

`magpie-protocol-detection-timeout.patch` 基于官方 `yetone/magpie` 的提交 `9dd7aabca8af76c9467039979076e58d59d3f03b`，仅修改协议检测：

- 单次和批量检测每次最多等待 60 秒。
- GUI 单模型总期限为 75 秒；批量按最多 30 模型、8 并发计算完整排队时间。
- 保留父 context 的取消能力，普通业务请求和端点测试期限保持原样。
- 同步子系统文档，并添加服务层与 GUI 回归测试。

## 应用

在对应版本的 **Magpie 源码目录**执行（不是 sub2api 目录）：

```powershell
git apply --check D:/workspace/sub2api/docs/patches/magpie-protocol-detection-timeout.patch
git apply D:/workspace/sub2api/docs/patches/magpie-protocol-detection-timeout.patch
go test -p 2 ./internal/provider ./internal/gui -run 'TestDetect|TestProviderDetect' -count=1
```

再按 Magpie 官方构建流程生成并替换安装程序。版本不同时先检查上下文，不能强行套用。此补丁不会通过部署 sub2api 自动改变已安装的 Magpie。

## 验证记录

- 真实 sub2api 的 Chat Completions / Anthropic Messages 请求均成功返回 200，未改变线上分组、模型或推理设置。
- 15 秒延迟模拟已复现原批量 12 秒失败，修复后通过。
- 最终使用 30 秒延迟模拟：原单次 20 秒与批量 12 秒均失败；修复后服务层和 GUI 路径均通过，覆盖 Chat 与 Anthropic，保留取消校验。
- 命令 `go test -p 2 ./internal/provider ./internal/gui -run 'TestDetect|TestProviderDetect' -count=1` 两包通过。
- 已检查补丁可应用性及格式；未运行 Magpie 全套跨平台构建、全套测试或发布，也未替换用户安装版本。
