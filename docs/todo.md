1）OmniLink 前端要能测通“基础群聊闭环”，后端必须实现哪些接口？

OmniLink 前端（ im.js ）已经按这些路径在调用；但当前后端实际只注册了私聊相关路由，群组相关路由在 https_server.go 里是注释掉的，同时 WS 也只支持私聊（ ws_handler.go ）。要让“创建群、邀请、同意进群、群消息收发、退群”跑通，建议后端至少补齐下面这些能力（按模块列）：

- 群组基础信息
  
  - POST /group/createGroup
    - 用途：创建群（群主=owner），生成群 ID（建议 G... ）
    - 请求： { owner_id, name, notice?, member_ids? }
    - 响应： data 至少返回 { group_id/uuid, name, notice, owner_id }
  - POST /group/getGroupInfo
    - 用途：群详情（群名/公告/群主/成员数/头像等）
    - 请求： { owner_id, group_id }
    - 响应： { group_id, name, notice, owner_id, member_cnt, avatar? }
  - POST /group/getGroupMemberList
    - 用途：群成员列表（用于群信息面板展示）
    - 请求： { owner_id, group_id }
    - 响应： [{ user_id, user_name/nickname, avatar }]
- 入群/邀请/同意（你提到“同意进群”，这里是关键差异点）
  
  - 最少两种实现路线（二选一，或都支持）：
    - A. 邀请即入群（最省事，但没有“同意进群”步骤）
      - POST /group/inviteGroupMembers
        - 请求： { owner_id, group_id, member_ids: [] }
        - 行为：直接把这些用户加入群成员，并为其创建/激活群会话
    - B. 邀请/申请需要审批（能覆盖“同意进群”）
      - POST /contact/applyContact
        - 请求： { owner_id, contact_id: group_id, contact_type: 1, message? }
        - 行为：创建“加群申请/群邀请”的申请单（pending）
      - POST /contact/getAddGroupList （群主或管理员查看待处理列表）
        - 请求： { group_id } 或 { owner_id: group_id } （你们风格二选一，统一即可）
        - 响应： [{ apply_id, user_id, nickname/username, avatar, message, created_at }]
      - POST /contact/passContactApply （同意进群）
        - 说明：OmniLink 当前实现里明确“暂不支持群组申请”，需要扩展它支持 contact_type=1 （见 contact_service.go ）
        - 请求： { apply_id, owner_id } ，其中 owner_id 对群申请应当是 group_id
        - 行为：把申请状态改为同意、写入群成员关系、创建/激活群会话
      - POST /contact/refuseContactApply （拒绝进群）
        - 请求同上： { apply_id, owner_id: group_id }
- 退群 / 解散
  
  - POST /group/leaveGroup
    - 请求： { owner_id: user_id, group_id }
    - 行为：移除成员关系（或标记退出状态），并让该用户的群会话不可再打开
  - POST /group/dismissGroup
    - 请求： { owner_id: group_owner_id, group_id }
    - 行为：解散群、清理/冻结群会话、成员关系、后续禁止发言/拉取历史等
- 会话（SessionList 要把私聊+群聊合并展示）
  
  - POST /session/getGroupSessionList
    - 请求： { owner_id }
    - 响应列表每项至少要满足前端归一化/展示需要： { session_id, group_id/peer_id, peer_name/group_name, peer_avatar/avatar, last_msg, updated_at, unread_count? }
  - POST /session/openSession
    - 请求： { send_id, receive_id } （receive_id 可能是 U... 或 G... ）
    - 行为：对群聊必须校验 send_id 是否已是群成员；不是成员则拒绝
  - POST /session/checkOpenSessionAllowed
    - 行为：对群聊同样要做“是否在群/群是否有效/是否被禁用”等校验
- 群消息历史
  
  - POST /message/getGroupMessageList
    - 请求： { group_id, page, page_size }
    - 响应：消息列表，至少包含： { uuid, session_id?, send_id, send_name, send_avatar, receive_id: group_id, type, content/url/file_*, created_at }
- WebSocket 群消息收发（基础群聊能不能跑通的核心）
  
  - 现状：OmniLink WS 只调用 SendPrivateMessage ，并且只 SendJSON(req.ReceiveId, ...) 给单个 client（ ws_handler.go ），这对 receive_id=G... 是不成立的。
  - 需要补齐：当 receive_id 是群 ID 时
    - 校验发送者是否为群成员、群是否存在/未解散/未禁用
    - 落库（群消息表或统一消息表标记 group_id）
    - 广播：给“所有在线群成员”逐个推送（通常也给发送者回显一份，保证前端能拿到 uuid/时间等服务端字段）
    - 会话更新：更新每个成员的会话 last_msg/updated_at（以及未读计数的策略：可在拉会话列表时计算或维护）
2）参考 KamaChat：群聊/私聊后端怎么实现？它们的区别与群聊重点在哪里？（文字说明）

核心数据与身份规则

- KamaChat 用前缀区分对象：用户 U... ，群 G... 。后端只看 receive_id 前缀就能决定是“单发”还是“群发”。
- 群的关键字段通常包括：群主（owner）、成员列表/成员关系、群状态（正常/禁用/解散）、入群模式（是否需要审批）。
私聊 vs 群聊：业务差异

- 关系模型不同
  - 私聊：前提是双方存在联系人关系（好友/未拉黑等），权限校验就是“你们能不能聊”。
  - 群聊：前提是“你是否是群成员”，权限校验是“你是否在群里 + 群是否有效 + 你是否被禁言/被踢”等。
- 消息投递目标不同
  - 私聊：1 条消息 → 1 个接收者（再加可选的发送者回显）。
  - 群聊：1 条消息 → N 个成员（在线就推送，不在线靠历史/未读补偿）。
- 会话与未读的计算更复杂
  - 私聊：每个用户只有一个对话对象，未读/最后消息更新很直观。
  - 群聊：同一群对每个成员都要维护“我的会话视角”（last_msg、更新时间、未读等），成员增减/退群会让会话权限与展示随之变化。
KamaChat 的“同意进群/审批”思路（可迁移到 OmniLink）

- KamaChat 把“加好友”和“加群”统一抽象成“申请（apply）”：
  - 申请记录里区分 contact_type ：用户 or 群
  - 状态：申请中/通过/拒绝/拉黑
- “需要审批的加群”一般流程：
  - 用户发起加群申请（或收到邀请产生一条申请记录）
  - 群主/管理员拉取待处理列表，选择同意/拒绝
  - 同意后：写入成员关系 + 更新群成员数 + 让该用户能拉到群/打开群会话
群聊 WebSocket 的工作原理 vs 私聊

- 私聊 WS：服务端收到消息 → 持久化 → 推送给接收者（如果在线）→ 通常给发送者也回显一份（保证前端拿到服务端生成的 uuid/时间等字段）。
- 群聊 WS：服务端收到消息 → 持久化 → 查询群成员列表 → 对每个成员做一次“在线推送”
  - 这一步本质是“fan-out（扇出）”：从 1 条输入消息扩散到 N 个连接
  - 重点在：成员查询效率、在线用户映射、广播的性能与失败处理（有人离线/连接断开等）
群组实现的重点（也是最容易踩坑的地方）

- 权限：任何群接口（拉历史、发消息、拉成员）都必须校验“是否群成员/是否有管理权限”。
- 成员关系：成员增删、退群、解散会影响会话列表、WS 广播对象、历史可见性。
- 广播与一致性：同一条群消息要保证“群内成员看到一致的消息 ID/时间/内容”，并正确落库以便离线补拉。
- 会话与未读：群会话通常是“每个成员一条会话记录（user_id + group_id）”，这样才能分别维护每个人的未读与最后消息。
如果你希望我把“OmniLink 后端当前缺口”按优先级映射成一个最小开发清单（先打通群 WS + 入群审批 + 会话列表），我也可以继续按你们现有模块结构（chat/contact/session/message）拆出每个接口应落在哪个 service/handler、需要哪些表字段与约束。

OmniLink 后端当前缺口（结论）

- WS 仅支持私聊 ：当前 WsHandler.Connect 无论 receive_id 是什么都走 SendPrivateMessage ，并且只 SendJSON(req.ReceiveId, ...) 给单个 user； receive_id=G... 时必然“发不出去/发错对象”。
- 入群审批链路不通 ： ApplyContact 已能识别 contact_id 前缀 G 并写申请（contact_type=1），但：
  - GetNewContactList 会过滤掉 ContactType != 0 （不返回群申请）
  - PassContactApply 明确 暂不支持群组申请 （ contact_service.go:L137-L139 ）
- 会话列表缺群会话接口 ：前端在调 /session/getGroupSessionList （ im.js ），后端路由未注册，且 SessionService 也只有 GetUserSessionList （ session_service.go ）。
- 群历史消息接口缺失 ： MessageService/MessageRepository 只有私聊查询（ message_repository.go ），无 GetGroupMessageList / ListGroupMessages 。
## 最小开发清单（按优先级，先打通：群 WS + 入群审批 + 会话列表）
### P0（必须先做，做完就能“看见群、能进群、能在群里发消息”） 1）chat：群 WS 收发（广播）——最关键阻塞点
改动点

- Handler：修改 ws_handler.go
  - 读到 SendMessageRequest 后：
    - if strings.HasPrefix(req.ReceiveId, "G") → 调用 RealtimeService.SendGroupMessage
    - else → 继续走现有 SendPrivateMessage
- Service：扩展 RealtimeService
  - 新增： SendGroupMessage(senderID string, req SendMessageRequest) ([]MessageItem, error) 或返回 (senderItem *MessageItem, receiverItems []MessageItem, memberIDs []string, error)
  - 逻辑要点：
    - 校验：群存在且 status=0；sender 是群成员（通过 user_contact 或 group_info.members）
    - 落库：message 表（ Message ）， receive_id=group_id ， session_id 可写 sender 对应的群 session（保证非空）
    - 广播：对所有群成员 hub.SendJSON(memberID, messageItem) （ Hub.SendJSON ）
    - 会话更新：对每个成员更新其 (send_id=member, receive_id=group) 的 session.last_message/last_message_at（复用 UpdateLastMessageBySendAndReceive ）
需要补的 repo 能力

- contactRepo：新增“按 group_id 列成员 user_id 列表”方法（从 user_contact 查 contact_type=1 且 status=0）
- 可选：groupRepo：按 group_id 取群信息（从 group_info ） 2）contact：入群审批（申请列表 + 同意/拒绝）
你要求“同意进群”，这里必须实现一条审批链路。建议做成 群专用接口 ，避免破坏现有好友申请 handler 里“强制 owner_id=当前登录用户”的逻辑（ contact_handler.go ）。

新增接口（建议路径与归属）

- Handler（contact 模块里新增 GroupHandler，路由挂 /group/* ）
  - POST /group/getJoinApplyList （群主/管理员查看待审批）
    - 请求： { group_id }
    - 鉴权：当前登录用户必须是群主（owner_id）或群管理员（如果你们要做管理员体系）
    - 输出：申请列表（apply_id、user_id、昵称、头像、message、时间）
  - POST /group/approveJoin （同意入群）
    - 请求： { group_id, apply_id }
    - 行为（事务）：
      1. apply 校验：contact_apply.uuid=apply_id，contact_id=group_id，contact_type=1，status=0
      2. 更新 apply.status=1
      3. 写入/更新 user_contact： (user_id=申请人, contact_id=group_id, contact_type=1, status=0)
      4. 更新 group_info：members/member_cnt（若 members 作为事实来源就要更新；否则可只更新 member_cnt）
      5. 创建群会话 session： (send_id=申请人, receive_id=group_id) ，ReceiveName/Avatar 来自 group_info
  - POST /group/rejectJoin （拒绝入群）
    - 请求： { group_id, apply_id } ，更新 apply.status=2
对现有接口的最小增强

- POST /contact/applyContact （已存在）：
  - 当前已经能识别 contact_id 前缀 G 并写 contact_apply.contact_type=1 （ contact_service.go ）
  - 但 缺少群存在/群状态校验 ：P0 需要补上（group_id 不存在要返回 404；群禁用/解散要 403/400） 3）session：群会话列表（前端 SessionList 合并展示依赖它）
新增接口

- POST /session/getGroupSessionList
  - Handler：在 chat/session_handler.go 新增方法并注册路由（参考 https_server.go 当前只注册了 getUserSessionList）
  - Service：在 SessionService 新增 GetGroupSessionList(ownerID string)
  - 实现方式（最小改动）：
    - 复用 repo： ListUserSessionsBySendID(ownerID) ，然后在 service 层过滤 receive_id 前缀 G
  - 输出字段：沿用现有 SessionItem （它已经能推断 peer_type=G，见 peerTypeOf ）
同时要补的校验

- OpenSession / CheckOpenSessionAllowed ：当 receive_id 是 G... 时必须校验“当前用户是否群成员 + 群状态正常”，否则任何人都能打开群会话（安全漏洞）
### P1（把“能用”补齐到“可完整测试群聊 UI/历史/群资料”） 4）message：群历史消息分页
新增接口

- POST /message/getGroupMessageList
  - Handler：新增到 message_handler.go
  - Service：MessageService 新增 GetGroupMessageList(req) ；Repo 新增 ListGroupMessages(groupID, page, pageSize)
  - 权限：调用者必须是群成员（查 user_contact contact_type=1 status=0）
  - 查询：建议按 receive_id=group_id + created_at desc 分页
注意点（与现有表结构的兼容）

- message 表的 session_id 非空，但群历史查询不应该依赖 “我的 session_id”，而应依赖 receive_id=group_id （KamaChat 也是按 group_id 聚合）。 5）contact：群资料/成员列表、我加入的群
这是为了让前端 GroupInfo/ContactList 显示完整。

- POST /contact/loadMyJoinedGroup
  - 返回：我加入的群列表（从 user_contact contact_type=1 status=0 反查 group_info）
- POST /group/getGroupInfo
- POST /group/getGroupMemberList
  - 成员列表建议从 user_contact 反查用户 brief，避免依赖 group_info.members JSON 一致性
## 表字段与约束（用你们现有表，最小增量）
现有表（已存在）

- group_info （ GroupInfo ）：uuid/name/notice/members(json)/member_cnt/owner_id/add_mode/status/created_at/updated_at
- user_contact （ UserContact ）：user_id/contact_id/contact_type/status
- contact_apply （ ContactApply ）
- session （ Session ）
- message （ Message ）
建议加的数据库约束/索引（保证一致性与性能，属于“群聊重点”）

- user_contact ：
  - 唯一约束： UNIQUE(user_id, contact_id, contact_type) ，防止重复入群/重复好友关系
  - 索引： INDEX(contact_id, contact_type, status) （群成员列表、权限校验会高频用）
- contact_apply ：
  - 索引： INDEX(contact_id, contact_type, status) （群待审批列表按 group_id 查 pending）
  - 可选唯一： UNIQUE(user_id, contact_id, contact_type) （保证“同一个人对同一个群只有一条有效申请”，你们 service 逻辑本身就是按这个维度 upsert）
- session ：
  - 唯一约束： UNIQUE(send_id, receive_id) （避免重复会话）
  - 索引： INDEX(send_id, last_message_at) （会话列表排序）
- message ：
  - 索引： INDEX(receive_id, created_at) （群历史分页）
  - 索引： INDEX(session_id, created_at) （私聊历史分页已在用）
关键不变量（实现时必须统一）

- ID 前缀：用户 U 、群 G 、会话 S 、消息 M 、申请 A （ util.GenerateGroupID/GenerateMessageID 等 已具备）
- “成员事实来源”二选一并保持同步：
  - 推荐：以 user_contact(contact_type=1) 为事实来源； group_info.members 作为缓存/冗余（可选）
  - 如果继续用 group_info.members ，那么 approve/leave/invite/dismiss 都必须同步更新 members + member_cnt（否则 WS 广播/成员列表会错）
如果你认可这个拆分，我可以再把每个 P0/P1 接口对应到“需要新增哪些 DTO（request/respond）与 repo 方法签名”，并把权限校验规则（群主、成员、禁用/解散）列成可直接照搬的判定表。