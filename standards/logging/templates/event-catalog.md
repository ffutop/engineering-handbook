# 日志事件清单（模板）

复制到项目的现行规范文档中。新增、修改或删除事件时，与代码在同一个 PR 中更新此表。示例行替换为项目自己的内容。

## 字段词表

同名字段在所有事件中类型与含义一致。

| 字段 | 类型 | 含义 |
|---|---|---|
| `err` | string | 错误信息 |
| `dur_ms` | int | 耗时（毫秒） |
| `payload` | string | 经白名单与脱敏允许的报文，遵守大小限制 |

## 事件

| event | 等级 | 主要字段 |
|---|---|---|
| `conn.closed` | INFO/WARN，按 `reason` | `reason`、`dur_ms`、`err`（异常时） |
| `request.handled` | INFO | 业务主键、`dur_ms` |
| `request.publish_failed` | ERROR | `err`、业务主键、补偿存储引用（如有） |

## 枚举取值

| 事件.字段 | 取值 | 等级 |
|---|---|---|
| `conn.closed.reason` | `completed`、`shutdown`、`idle_timeout`、`eof` | INFO |
| `conn.closed.reason` | `reset`、`read_error`、`parse_error` | WARN |

## 数据与容量策略

填写字段白名单、敏感字段处理、单条大小上限、截断标识、保留期、访问责任人及采样策略；未声明可输出的原始报文默认不输出。
