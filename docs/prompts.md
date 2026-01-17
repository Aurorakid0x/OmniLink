- 你是资深 Golang 工程师。项目在 c:\Users\chenjun\goProject\OmniLink ，遵循 DDD 分层（domain/application/infrastructure/interface）。我处于 Chat Mode（只读），你需要：先用检索/阅读梳理现有架构、相关模块依赖与代码风格；再给出改动方案；对每个已存在文件的修改必须先给出 diff 预览；新文件直接给完整内容。
- 需求：{把你的需求写这里，包含接口、字段、错误码、权限等}
- 约束：不引入新第三方库（除非项目已使用）；敏感信息不得出现在返回/日志；遵循现有 xerr/back/zlog 习惯；提供必要的单元测试或最小可运行验证步骤（命令以 Windows 为准）。





现在 OmniLink 中存在 {描述 bug/性能问题/代码坏味道}。请先定位触发路径（从 interface→application→domain→infrastructure），给出最小改动方案并实现。要求：不改动公共行为（除非说明），新增逻辑要有测试或可复现步骤；所有修改按文件 diff 预览输出。

- 你在 Builder 模式，拥有写文件与运行命令能力。请在 c:\Users\chenjun\goProject\OmniLink 按 DDD 分层新增功能：{需求}。
- 必做：
- interface/http：新增 handler/路由（遵循现有 gin 风格）；
- application：新增 service 与 DTO request/response；
- domain：补齐 entity/repository interface（如需要）；
- infrastructure：实现 gorm repository；
- 鉴权：如果需要登录态，复用现有 jwt middleware；
- 验证：本地跑 go test ./... （或给出能跑通的最小命令），并提供一个 curl/Postman 示例（Windows 友好）。
- 约束：不引入新库（除非已存在）；敏感字段永不返回；错误处理用 xerr/back 。




- 你在 Builder 模式。前端在 c:\Users\chenjun\goProject\OmniLink\web （Vue 项目）。请实现功能：{需求，例如“好友详情弹窗/群成员搜索/会话列表增强”}。
- 必做：
- 先阅读现有组件与请求封装（如 web\src\utils\request.js 、 web\src\api\im.js ），沿用现有代码风格；
- 新增/修改组件（如 web\src\components\chat\... ）；
- 若需要路由/状态管理，按现有 router/store 写法接入；
- 用 npm install / npm run dev （Windows）跑起来并给出访问路径与验证步骤；
- 不新增依赖（除非项目已用）。


todo：
- 梳理项目
- 把现有同步 /ai/internal/rag/backfill 改成“创建 ai_backfill_job + 写入多条 ai_ingest_event （pending）”，由 Outbox Relay 投递 Kafka
- 实现 Kafka consumer worker：从 omnilink.ai.ingest 消费 event_id ，回查 DB 拿 payload，然后调用 reader + pipeline.Ingest，再更新 ai_ingest_event.status 和 ai_backfill_job 统计