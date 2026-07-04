# Agent Flow 文档

Agent Flow 是基于 Kubernetes 和 eino 的 AI Agent 编排系统，采用 Planner-Worker-Monitor 三层架构，并支持 Workflow 多步骤流水线。

## 文档目录

| 文档 | 说明 |
|------|------|
| [架构设计](architecture.md) | 系统架构、组件职责、数据流 |
| [快速开始](getting-started.md) | 部署和使用指南 |
| [Workflow 编排](workflow.md) | 100 章小说、重试策略、运维脚本 |
| [API 参考](api-reference.md) | HTTP API 接口 |
| [配置指南](configuration.md) | AI、日志、重试、环境变量 |
| [部署指南](deployment.md) | 部署架构、CI、故障排查 |
| [CRD 参考](crd-reference.md) | Task、Workflow、Monitor CRD |

## 核心概念

### 三层架构

- **Planner**：分析需求、协调 Task/Workflow 生命周期
- **Worker**：执行任务，生成分段章节等产出物
- **Monitor**：评估质量，未通过则带反馈重试

### Workflow 流水线（默认团队模式）

```
历史调研(可选) → 大纲 → 设定圣经 → 逐章[梗概 → 剧情 → 执笔者 → 润色 → 质检] → 合并书稿
```

另支持：**导入拆书**（`novel-import-deconstruct`）、**选章重写**（`novel-chapter-rewrite`）、**RAG 剧情检索**。

### 重试（指数退避）

| 层级 | 触发 | 默认上限 |
|------|------|----------|
| 分段 | 单段字数不足 | 3 次/段 |
| Task | 校验/质量不通过 | 章节 6 次 |
| Workflow | Task 终态失败 | 3 次重派 |

### 技术栈

Go · eino · Kubernetes · slog · React · SQLite · agent-sandbox

## 快速链接

- [项目 README](../README.md)
- [Makefile](../Makefile)
- [CI 工作流](../.github/workflows/ci.yml)
- [小说 Workflow 样本](../config/samples/novel_parallel_demo.yaml)
- [AI 配置](../config/ai_config.yaml)