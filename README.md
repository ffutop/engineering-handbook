# Engineering Handbook

跨项目工程规范，以可验证的最低要求约束协作，并把业务相关的架构取舍留给项目。

| 规范 | 保护的结果 |
|---|---|
| [规范治理](standards/documentation/standard-governance.md) | 明确适用范围、规则强度、采用版本与例外 |
| [文档治理](standards/documentation/document-governance.md) | 规则来源唯一，决定、实现与验证可区分 |
| [Git 提交信息](standards/git/commit-messages.md) | 变更意图清晰，提交保持单行并兼容既有规则 |
| [Go 结构化日志](standards/logging/go-structured-logging.md) | 可检索、可定位、无泄露，成本可控 |
| [Flyway 数据库迁移](standards/database/flyway-migration.md) | 历史可追溯，升级兼容，失败可恢复 |
| [GitHub 发版](standards/release/github-release.md) | 版本与制品可追溯，部分发布可恢复 |

采用项目在 `ENGINEERING_STANDARDS.md` 中固定标准版本并声明项目策略。模板须适配后验证，具体能力和限制以对应规范为准。未发布修订须经过维护者评审后发布，不能直接当作已生效的项目约束。
