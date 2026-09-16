# Doris exporter NaN/Inf upstream status

调查日期：2026-09-16（Asia/Shanghai）
范围：官方 `open-telemetry/opentelemetry-collector-contrib` GitHub release、API、源码、issue 和 PR。未修改产品代码。

## 结论

截至调查日期，问题**尚未进入已发布版本**：最新 Contrib release 是 `v0.161.0`，但该版本及当前 `main` 的 Doris metrics exporter 仍会把包含 `NaN`、`+Inf` 或 `-Inf` 的 `float64` 交给 Go `encoding/json`，导致 `json: unsupported value: NaN` 类似的序列化失败。

上游已有对应 issue 和修复 PR，但二者均未闭合/合并：

- Issue [#50569](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/50569) 仍为 `open`，标题为 `[exporter/doris] Metrics export fails when datapoint contains NaN or Inf`。
- PR [#50611](https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/50611) 仍为 `open`、`merged=false`，当前标签包含 `waiting-for-code-owners`；其 head commit 为 [`8d3a820a597d6f5da4e158a6ccafae1a33726901`](https://github.com/open-telemetry/opentelemetry-collector-contrib/commit/8d3a820a597d6f5da4e158a6ccafae1a33726901)。
- 另一个 PR [#50593](https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/50593) 已关闭，不能视为已发布修复。

因此，仅升级到当前最新正式版 `v0.161.0` 不能解决该问题；需要等待 #50611 或后续修复合并并进入 release，或在本地/自定义发行版应用等价修复。

## 最新版本证据

- GitHub latest release API：[releases/latest](https://api.github.com/repos/open-telemetry/opentelemetry-collector-contrib/releases/latest) 返回 `tag_name=v0.161.0`、`published_at=2026-09-15T15:05:47Z`，对应页面：[Release v0.161.0](https://github.com/open-telemetry/opentelemetry-collector-contrib/releases/tag/v0.161.0)。
- 官方 tags API：[tags](https://api.github.com/repos/open-telemetry/opentelemetry-collector-contrib/tags?per_page=10) 返回 `v0.161.0`，tag commit 为 [`3f8455d8038a985398861171e5310bc9b4e988b2`](https://github.com/open-telemetry/opentelemetry-collector-contrib/commit/3f8455d8038a985398861171e5310bc9b4e988b2)。Release 页面显示其版本提交为 [`982f20b8a8e8a2569fab3e27cf8b008e8a5080c1`](https://github.com/open-telemetry/opentelemetry-collector-contrib/commit/982f20b8a8e8a2569fab3e27cf8b008e8a5080c1)；这里保留 API 与 release 页面给出的两个官方引用值，不据此推断额外行为。

## 当前源码证据

在调查时，官方 `main` ref 为 [`75c6ba5d9d5a699c487cc6859319e55e9bfa8dcd`](https://github.com/open-telemetry/opentelemetry-collector-contrib/commit/75c6ba5d9d5a699c487cc6859319e55e9bfa8dcd)。相关源码仍是：

1. [`exporter/dorisexporter/exporter_common.go`](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/exporter/dorisexporter/exporter_common.go#L146-L155) 的 `toJSONLines` 创建 `json.NewEncoder`，逐条 `Encode`，遇到错误直接返回。
2. [`exporter/dorisexporter/exporter_metrics.go`](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/exporter/dorisexporter/exporter_metrics.go#L227-L235) 的 `pushMetricDataInternal` 先调用 `metrics.bytes()`；编码失败时直接返回，因此不会发起 Doris Stream Load 请求。
3. 五种 metrics model 仍使用 JSON `float64` 字段：
   - [`metrics_gauge.go`](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/exporter/dorisexporter/metrics_gauge.go#L16-L24)
   - [`metrics_sum.go`](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/exporter/dorisexporter/metrics_sum.go#L16-L25)
   - [`metrics_histogram.go`](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/exporter/dorisexporter/metrics_histogram.go#L16-L29)
   - [`metrics_exponential_histogram.go`](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/exporter/dorisexporter/metrics_exponential_histogram.go#L16-L34)
   - [`metrics_summary.go`](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/exporter/dorisexporter/metrics_summary.go#L16-L30)

以上结构包含 datapoint value、histogram sum/min/max/bounds、exponential histogram sum/min/max/zero threshold、summary sum/quantile 以及 exemplar value；当前源码没有针对 non-finite 值的过滤或替换逻辑。

## 上游修复状态

PR [#50611](https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/50611) 的描述和 diff 表明其拟议行为是：对五类 metric 丢弃带 `NoRecordedValue` 或 non-finite 值的 datapoint，逐个过滤 non-finite exemplar，并继续发送同一批次中的有效 datapoint；同时增加 `.chloggen/fix_50569-doris-metrics-nan-inf.yaml`。该改动只存在于 PR head，不在 `v0.161.0` 或当前 `main`。

PR [#50611 files](https://api.github.com/repos/open-telemetry/opentelemetry-collector-contrib/pulls/50611/files?per_page=100) 可直接核对改动；其 GitHub API 状态为 `state=open`、`merged_at=null`，并请求 `atoulme`、`joker-star-l` review。PR check runs 已有成功项，但这不等同于合并或 release。

PR [#50593](https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/50593) 曾提出相近的“丢弃 non-finite 值”方案，但 GitHub API 显示 `state=closed`、`merged_at=null`，所以不能作为已解决证据。

## 复核边界

- 本记录只使用 OpenTelemetry 官方 GitHub 仓库页面/API、官方源码和官方 release 页面。
- 未以 issue/PR 的测试声明替代已发布源码证据。
- 未进行真实 Doris 集群验收；本结论只回答“最新上游版本是否已经包含修复”。
