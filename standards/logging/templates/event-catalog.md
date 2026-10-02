# 日志事件清单（模板）

复制到项目的现行规范文档中。新增、修改或删除事件时，与代码在同一个 PR 中更新此表。示例行替换为项目自己的内容。

## 字段词表

同名字段在所有事件中类型与含义一致。

| 字段 | 类型 | 含义 |
|---|---|---|
| `err` | string | 错误信息 |
| `dur_ms` | int | 耗时（毫秒） |
| `payload` | string | 原始报文 |

## 事件

| event | 等级 | 主要字段 |
|---|---|---|
| `conn.closed` | INFO/WARN，按 `reason` | `reason`、`dur_ms`、`err`（异常时） |
| `request.handled` | INFO | 业务主键、`dur_ms` |
| `request.publish_failed` | ERROR | `err`、`payload` |

## 枚举取值

| 事件.字段 | 取值 | 等级 |
|---|---|---|
| `conn.closed.reason` | `completed`、`shutdown`、`idle_timeout`、`eof` | INFO |
| `conn.closed.reason` | `reset`、`read_error`、`parse_error`、`panic` | WARN |
