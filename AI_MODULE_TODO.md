# AI 模块改进计划 (TODO)

本文档记录了对 OmniLink AI 模块的代码 Review 发现的问题及改进建议。

## 1. 任务执行流水线 (Job Execution Pipeline)

- [ ] **工具动态加载**：目前 `JobExecutionPipeline` 在启动时加载所有工具。如果未来工具有针对用户的权限控制（如某些工具只能被特定用户使用），需要在 Pipeline 运行时动态过滤工具，而不是使用全局 `allTools`。
- [ ] **并发安全**：`SetTools` 方法目前会修改 Pipeline 实例的 `tools` 字段。虽然目前应用初始化后不会变更，但如果支持热加载工具，需要加锁保护。
- [ ] **重试机制优化**：目前任务失败重试是基于 Scheduler 的简单重试。建议在 Pipeline 内部增加针对特定错误（如网络超时）的重试机制，避免整个任务重跑。

## 2. 消息通知 (Notification Handler)

- [ ] **事务一致性**：`handlePushNotification` 中，保存消息到数据库和推送到 WebSocket 是两个独立操作。如果保存成功但推送失败（或反之），会导致状态不一致。建议引入消息队列或发件箱模式（Outbox Pattern）保证一致性。
- [ ] **Agent 信息获取**：目前推送消息中的 `send_name` 硬编码为 "AI助手"。应该从 `AgentRepository` 获取真实的 Agent 名称和头像。

## 3. 调度器 (Scheduler)

- [ ] **分布式锁**：`SchedulerManager` 目前是单机运行。如果部署多实例，会导致任务重复执行。需要引入 Redis 分布式锁或使用专门的分布式调度系统（如 Temporal/Asynq）。
- [ ] **Cron 表达式校验**：创建任务时的 Cron 表达式校验可以更严格，避免无效表达式进入数据库。

## 4. 前端适配 (Web)

- [ ] **WebSocket 重连**：前端 WebSocket 断线重连逻辑较为简单，建议增加更健壮的重连策略和离线消息同步机制。
- [ ] **消息类型扩展**：目前 `ai_notification` 较为通用，未来可以区分 `ai_alert`（报警）、`ai_report`（日报）等不同类型，前端展示不同 UI。

## 5. 代码结构

- [ ] **Repo 依赖注入**：`https_server.go` 中的依赖注入逻辑较为臃肿，建议使用 Wire 等依赖注入框架简化代码。
- [ ] **错误处理**：部分 Log 记录的错误信息可以更详细，包含 Stack Trace 以便排查问题。
