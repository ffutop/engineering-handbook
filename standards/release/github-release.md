# GitHub 发版标准

本标准约束托管在 GitHub、使用 GitHub Actions 发版的团队项目。它规定一次发版从 Git tag 到 GitHub Release、容器镜像和 Helm chart 的最低要求；构建工具、目标平台、镜像仓库和审批人由每个项目在 `ENGINEERING_STANDARDS.md` 中声明。

参考实现：

- [`templates/release.yml`](templates/release.yml)：发版流水线，复制到 `.github/workflows/release.yml`；
- [`templates/release-notes.yml`](templates/release-notes.yml)：发布说明分类，复制到 `.github/release.yml`。

## 1. 项目采用声明

采用本标准的项目在 `ENGINEERING_STANDARDS.md` 中至少声明：

| 字段 | 要求 |
|---|---|
| 标准地址与版本 | 指向本文件，版本为 handbook 已发布的 Git tag |
| 交付物 | 文件制品 / 容器镜像 / Helm chart 中的哪几种 |
| 构建契约 | 产出文件制品的命令，默认 `make release-artifacts VERSION TARGET OUT` |
| 目标平台 | 文件制品的 OS/架构矩阵；镜像的 `platforms` |
| 镜像与 chart 仓库 | 默认 `ghcr.io/<owner>/<repo>` 与 `oci://ghcr.io/<owner>/charts` |
| 维护分支 | 除默认分支外允许打 tag 的分支，默认 `release/*` |
| 发版负责人 | 可创建 `v*` tag 的人员或团队，以及 `release` 环境的审批人 |

## 2. 发版流程

```text
打 annotated tag vX.Y.Z 并推送
  └─ prepare   校验 tag，计算版本号，探测交付物
       ├─ build    构建文件制品          ┐
       ├─ image    构建镜像，只按 digest 推送 ├ 此阶段对外不可见
       └─ chart    lint 并打包 chart       ┘
            └─ release  [release 环境审批]
                 生成 SHA256SUMS 与来源证明 → 创建草稿 Release 并上传资产
                 → 镜像打 tag → 推送 chart → 公开 Release
```

发版人只做一件事：在已合入的提交上创建并推送 tag。

```bash
git tag -s v1.4.0 -m "v1.4.0"   # 未配置签名时使用 -a
git push origin v1.4.0
```

## 3. 强制规则

### 3.1 tag 是唯一触发源和版本来源

- 发版只由推送 `v` 开头的 tag 触发；禁止通过手动触发、分支推送或网页创建 Release 发版。
- tag 格式为 `vMAJOR.MINOR.PATCH[-PRERELEASE]`，遵循 SemVer 2.0；禁止 build metadata（`+xxx`），因为它不能用作镜像 tag。
- 带 `-` 后缀的版本（如 `v1.4.0-rc.1`）一律作为预发布：Release 标记为 Pre-release，不成为 Latest，镜像不更新浮动 tag。
- 版本号只从 tag 取得，并在构建时注入二进制、镜像（`VERSION` build-arg）和 chart（`--version`、`--app-version`）。仓库内的 `Chart.yaml`、`package.json` 等版本字段不作为发版依据，可保留占位值。

### 3.2 tag 必须可追溯

- 必须是 annotated tag，推荐签名 tag。lightweight tag 会被流水线拒绝。
- tag 指向的提交必须已存在于默认分支或声明的维护分支上，即已经过 PR 评审和必需检查。流水线不重复执行完整测试，因此分支保护是发版质量的前提。
- 通过仓库 Ruleset 保护 `refs/tags/v*`：仅发版负责人可创建，禁止更新和删除。

### 3.3 已发布版本不可变

- 已公开的 Release、已推送的版本 tag 和不可变镜像 tag（`X.Y.Z`）不得修改、覆盖或删除。发现问题时发布新的 patch 版本向前修复。
- 仓库开启 GitHub Immutable Releases。流水线先创建草稿、上传全部资产，最后才公开，以适配该设置。
- 重跑失败的发版只允许在草稿阶段；Release 一旦公开，流水线拒绝再次发布同一 tag。重跑时应使用「Re-run failed jobs」，复用已构建的制品，保证推送内容与校验和一致。

### 3.4 构建一次，审批后统一对外

- 每种交付物在一次运行中只构建一次；对外发布的必须是被计入校验和、被证明的同一份字节。
- 审批前，镜像只按 digest 推送、不打 tag，chart 和文件只作为 workflow artifact 存在，均对外不可见。
- 对外可见的动作全部集中在 `release` job，由 GitHub Environment `release` 的 required reviewers 放行。

### 3.5 交付物规范

| 交付物 | 要求 |
|---|---|
| 文件制品 | 命名 `<name>_<version>_<os>_<arch>.<ext>`；附 `SHA256SUMS` |
| 容器镜像 | 多架构 index；稳定版 tag 为 `X.Y.Z`、`X.Y`、`X`（`0.x` 不打 `X`）和 `latest`，预发布只打 `X.Y.Z-pre`；OCI labels 含源码仓库与 revision；附 SBOM 与 provenance |
| Helm chart | `helm lint --strict` 通过；以 OCI 形式推送；chart version 与 appVersion 都等于 `X.Y.Z`；镜像 tag 默认取 `.Chart.AppVersion`；`.tgz` 同时作为 Release 资产 |

部署时引用镜像应使用 digest 或 `X.Y.Z`；`latest`、`X`、`X.Y` 仅供人工试用。

### 3.6 供应链安全

- workflow 顶层 `permissions: {}`，每个 job 只申请所需权限；写权限（`contents`、`packages`）只出现在发布相关 job。
- 所有第三方 action 固定到完整 commit SHA，并以注释标明版本；由 Dependabot 或 Renovate 维护升级。
- 使用内置 `GITHUB_TOKEN` 发布到 GitHub Releases 与 GHCR，不使用个人令牌。推送到外部仓库的凭据只存放在 `release` 环境的 secrets 中。
- 文件制品与镜像均生成 GitHub artifact attestation，用户可通过 `gh attestation verify` 验证来源。私有仓库需 GitHub Enterprise Cloud 才能使用该功能；不满足时须在采用声明中说明替代方案（如 cosign 签名）。
- checkout 设置 `persist-credentials: false`。

### 3.7 发布说明

使用 GitHub 自动生成的发布说明，按 PR 标签分类（见 [`templates/release-notes.yml`](templates/release-notes.yml)）。流水线在其前面附加制品清单：镜像引用与 digest、chart 安装命令、校验与验证方法。破坏性变更必须带 `breaking-change` 标签，并在 PR 描述中写明迁移步骤。

## 4. 项目必须决定的策略

以下不是本标准的默认结论，项目必须在采用声明或 ADR 中作出选择：

- 是否维护多个版本线，以及维护分支的建立与终止规则；
- 预发布阶段划分（`alpha` / `beta` / `rc`）及其进入条件；
- 镜像与 chart 发布到 GHCR 以外仓库时的凭据和同步方式；
- 发版后的部署是否自动触发，以及由哪条流水线负责；
- 是否使用独立的 CHANGELOG 文件，若使用，它与自动发布说明的关系。

## 5. 评审与验证

- 修改 `.github/workflows/release.yml` 必须经过发版负责人评审，并通过 `actionlint`。
- 首次采用或修改流水线后，先推送一个预发布 tag（如 `v0.0.0-rc.1`）完成全流程验证，确认草稿、审批、镜像 tag、chart 推送与公开各步骤行为正确。
- 每次发版后，发版负责人确认：Release 资产完整、`sha256sum -c SHA256SUMS` 通过、`gh attestation verify` 通过、镜像与 chart 可按发布说明中的命令拉取。

## 6. 相关资料

- [GitHub Actions：工作流语法](https://docs.github.com/actions/writing-workflows/workflow-syntax-for-github-actions)
- [GitHub：Immutable releases](https://docs.github.com/code-security/supply-chain-security/understanding-your-software-supply-chain/immutable-releases)
- [GitHub：Artifact attestations](https://docs.github.com/actions/security-for-github-actions/using-artifact-attestations)
- [GitHub：自动生成发布说明](https://docs.github.com/repositories/releasing-projects-on-github/automatically-generated-release-notes)
- [Semantic Versioning 2.0.0](https://semver.org/)
- [Helm：OCI registries](https://helm.sh/docs/topics/registries/)
