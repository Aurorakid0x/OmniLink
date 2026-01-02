### 全流程推演：A 给 B 发消息 -> B 回复 A 场景 0：准备阶段（管道早已建立）
- A 的动作 ：打开 APP，前端自动请求 GET /wss?client_id=A 。
  
  - 后端代码 ( ws_handler.go ):
    - Connect 被调用。
    - conn, err := upgrader.Upgrade(...) -> A 的管道建立成功 。
    - client := ws.NewClient("A", conn) -> 创建 A 的 Client 对象。
    - h.hub.Register(client) -> 记录“A 在线”。
    - go client.WritePump() -> A 的写管道（发货员）就位 。
    - for { conn.ReadJSON(...) } -> A 的读管道（收货员）就位，开始死循环等待 。
- B 的动作 ：同上。B 也请求了 /wss?client_id=B 。
  
  - 后端状态 ： Hub.clients 里现在有两个记录： "A": {ClientA}, "B": {ClientB} 。 阶段 1：A 发出第一条消息 “Hello”
1. A 的前端 通过 WebSocket 发送 JSON： {"receive_id": "B", "content": "Hello", "type": 1} 。

2. 后端接收（ws_handler.go）

- 代码 ： if err := conn.ReadJSON(&req); (Line 84)
- 动作 ：A 的死循环苏醒，读取到这条 JSON，反序列化到 req 变量中。
3. 业务处理（realtime_service.go）

- 代码 ： h.svc.SendPrivateMessage("A", req) (Line 88)
- 动作 ：
  - 检查 A 和 B 是不是好友。
  - 存库 ： messageRepo.Create(msg) -> 把 "Hello" 写入 MySQL。
  - 返回 ：生成两个对象：
    - senderItem : 给 A 看的（带发送时间）。
    - receiverItem : 给 B 看的。
4. 转发给 A（给自己个回执）

- 代码 ： h.hub.SendJSON("A", senderItem) (Line 97)
- 内部流程 ( hub.go Line 54 Send ):
  - set := h.clients["A"] -> 查表，找到 A 的 Client 对象。
  - c.send <- payload -> 把数据塞进 A 的 send 管道（内存通道）。
  - A 的 WritePump ( hub.go Line 119): 监听到 send 管道有数据 -> conn.WriteMessage -> 数据通过网络发回给 A 。
  - A 的前端 ：收到回执，把消息状态从“发送中...”变成“发送成功”。
5. 转发给 B（核心：B 怎么收到？）

- 代码 ： h.hub.SendJSON("B", receiverItem) (Line 98)
- 内部流程 ( hub.go Line 54 Send ):
  - set := h.clients["B"] -> 查表！因为 B 早就连上了，所以能找到 B 的 Client 对象。
  - c.send <- payload -> 把数据塞进 B 的 send 管道。
  - B 的 WritePump ( hub.go Line 119): B 的那个协程一直在后台挂起，现在突然发现管道里有货了！
  - conn.WriteMessage(...) -> 数据通过 B 的 WebSocket 连接推送到 B 的手机 。
  - B 的前端 ：收到推送，界面上弹出一个新气泡 “Hello”。 阶段 2：B 回复 “Hi”
这个过程其实和上面完全一样，只是角色互换。

1. B 的前端 发送 {"receive_id": "A", "content": "Hi", ...} 。

2. 后端接收（ws_handler.go）

- 注意 ：这次触发的是 B 的那个死循环 ( for { conn.ReadJSON } )。
- 代码 ： conn.ReadJSON(&req) 读到了 "Hi"。
3. 业务处理

- 代码 ： SendPrivateMessage("B", req) 。
- 动作 ：存库 "Hi"，生成 senderItem (给 B), receiverItem (给 A)。
4. 转发给 B（回执）

- h.hub.SendJSON("B", senderItem) -> B 的 WritePump 工作 -> B 看到“发送成功”。
5. 转发给 A（收到回复）

- h.hub.SendJSON("A", receiverItem) -> 查表找到 A -> A 的 WritePump 工作 -> A 收到 "Hi"。