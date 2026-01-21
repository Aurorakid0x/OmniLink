# AI Assistant Testing Guide

This guide provides comprehensive testing instructions for the OmniLink Global AI Assistant feature.

## Prerequisites

### 1. Database Setup
Ensure MySQL is running and the database tables are created:
```bash
# The tables will be auto-migrated on first run:
# - ai_assistant_session
# - ai_assistant_message  
# - ai_agent
```

### 2. Configuration
Verify `configs/config_local.toml` has proper AI provider settings:
```toml
[aiConfig.embedding]
provider = "dashscope"  # or "ark"
apiKey = "your-api-key"
model = "text-embedding-v4"

[aiConfig.chatModel]
provider = "ark"  # or "openai"
apiKey = "your-api-key"
model = "doubao-seed-1-6-flash-250828"
```

### 3. Start Backend Server
```bash
cd /path/to/OmniLink
go run ./cmd/OmniLink/main.go
```

Server should start on `http://localhost:8000`

---

## Backend API Testing

### Test 1: Get Sessions List (Empty State)

**Request:**
```bash
curl -X GET http://localhost:8000/ai/assistant/sessions \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

**Expected Response:**
```json
{
  "code": 200,
  "message": "success",
  "data": []
}
```

---

### Test 2: Get Available Agents

**Request:**
```bash
curl -X GET http://localhost:8000/ai/assistant/agents \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

**Expected Response:**
```json
{
  "code": 200,
  "message": "success",
  "data": []
}
```
*Note: Currently no agents are seeded. This is expected.*

---

### Test 3: Chat (Non-Streaming) - First Message

**Request:**
```bash
curl -X POST http://localhost:8000/ai/assistant/chat \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{
    "question": "什么是人工智能?",
    "top_k": 5
  }'
```

**Expected Response Structure:**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "session_id": "AS12345678901",
    "answer": "人工智能（AI）是...",
    "citations": [
      {
        "chunk_id": "C...",
        "source_type": "chat_private",
        "source_key": "S...",
        "content": "相关知识内容...",
        "score": 0.89
      }
    ],
    "timing": {
      "embedding_ms": 120,
      "search_ms": 45,
      "post_process_ms": 10,
      "llm_ms": 1500,
      "total_ms": 1675
    },
    "tokens": {
      "prompt_tokens": 256,
      "answer_tokens": 128,
      "total_tokens": 384
    }
  }
}
```

**Verification Checklist:**
- ✅ `session_id` starts with "AS" and is 20 chars
- ✅ `answer` contains AI-generated content
- ✅ `citations` array is present (may be empty if no RAG data)
- ✅ `timing` shows reasonable milliseconds
- ✅ `tokens` counts are positive integers

---

### Test 4: Chat (Non-Streaming) - Follow-up Message

**Request:**
```bash
curl -X POST http://localhost:8000/ai/assistant/chat \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{
    "session_id": "AS12345678901",
    "question": "请详细解释一下"
  }'
```

**Expected Behavior:**
- ✅ Same `session_id` returned
- ✅ Response references previous conversation context
- ✅ Message count in session increases

---

### Test 5: Chat Stream (SSE) - Real-time Streaming

**Request:**
```bash
curl -X POST http://localhost:8000/ai/assistant/chat/stream \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{
    "question": "用200字解释量子计算",
    "top_k": 3
  }'
```

**Expected SSE Stream:**
```
event: delta
data: {"content":"量子"}

event: delta
data: {"content":"计算"}

event: delta
data: {"content":"是..."}

event: done
data: {"session_id":"AS12345678902","citations":[...],"tokens":{...}}
```

**Verification Checklist:**
- ✅ Headers include `Content-Type: text/event-stream`
- ✅ Multiple `delta` events received with incremental content
- ✅ Final `done` event contains full metadata
- ✅ No `error` events

---

### Test 6: Error Handling - Missing Question

**Request:**
```bash
curl -X POST http://localhost:8000/ai/assistant/chat \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{
    "question": ""
  }'
```

**Expected Response:**
```json
{
  "code": 400,
  "message": "参数错误"
}
```

---

### Test 7: Error Handling - Invalid Session ID

**Request:**
```bash
curl -X POST http://localhost:8000/ai/assistant/chat \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{
    "session_id": "INVALID_ID",
    "question": "测试"
  }'
```

**Expected Response:**
```json
{
  "code": 404,
  "message": "会话不存在"
}
```

---

### Test 8: Error Handling - Unauthorized

**Request:**
```bash
curl -X POST http://localhost:8000/ai/assistant/chat \
  -H "Content-Type: application/json" \
  -d '{
    "question": "测试"
  }'
```

**Expected Response:**
```json
{
  "code": 401,
  "message": "未登录"
}
```

---

## Frontend Testing

### Prerequisites

1. Start frontend dev server:
```bash
cd web
npm run dev
```

2. Open browser: `http://localhost:5173`
3. Login with valid credentials

---

### Test 1: Navigation to Assistant Page

**Steps:**
1. Click the **MagicStick** icon in the left sidebar
2. Verify URL changes to `/assistant`
3. Verify page loads without errors

**Expected UI:**
- ✅ Left panel shows "AI 助手" title
- ✅ Right panel shows "AI 个人助手" header
- ✅ Purple/blue gradient styling visible
- ✅ "暂无会话" empty state displayed (if no sessions)

---

### Test 2: Create New Chat Session

**Steps:**
1. Click the **Plus** button in session list header
2. Type question in input area: "你好，请介绍一下你自己"
3. Press **Enter** to send

**Expected Behavior:**
- ✅ Input cleared immediately
- ✅ User message appears on the right (with your avatar)
- ✅ "AI 正在思考..." indicator appears
- ✅ AI response streams in token-by-token (purple bubble on left)
- ✅ New session appears in left panel session list
- ✅ Session is auto-selected (highlighted)

---

### Test 3: Citations Display

**Prerequisites:** 
- Send a question that triggers RAG retrieval (e.g., "OmniLink项目的架构是什么?")

**Steps:**
1. Wait for AI response to complete
2. Look for collapsible "引用来源" section below AI message

**Expected Behavior:**
- ✅ Citation count badge shows (e.g., "引用来源 (3)")
- ✅ Click to expand citation list
- ✅ Each citation card shows:
  - Source type tag (e.g., "chat_private")
  - Similarity score (e.g., "相似度: 89.5%")
  - Content excerpt
  - Metadata (source_key, chunk_id)

---

### Test 4: Session Switching

**Prerequisites:** 
- Create 2+ chat sessions

**Steps:**
1. Click on a different session in the left panel
2. Verify right panel updates

**Expected Behavior:**
- ✅ Selected session highlighted with purple gradient
- ✅ Message history loads for selected session
- ✅ Previous conversation context visible
- ✅ Input area remains enabled

---

### Test 5: Agent Selection

**Steps:**
1. Click the agent dropdown in header
2. Verify dropdown shows available agents

**Expected Behavior:**
- ✅ Dropdown opens without error
- ✅ If agents exist, they're listed
- ✅ If no agents, dropdown is empty (expected currently)
- ✅ Selection persists for new messages

---

### Test 6: SSE Streaming Visual Feedback

**Steps:**
1. Send a long question: "请详细介绍人工智能的历史发展，包括重要里程碑事件"
2. Observe streaming behavior

**Expected Behavior:**
- ✅ Response appears incrementally (not all at once)
- ✅ Scroll auto-follows to bottom during streaming
- ✅ Loading spinner disappears when stream completes
- ✅ Input re-enabled after completion

---

### Test 7: Input Handling - Keyboard Shortcuts

**Steps:**
1. Type multi-line text in input
2. Press **Shift+Enter**
3. Press **Enter**

**Expected Behavior:**
- ✅ Shift+Enter: Inserts newline (does NOT send)
- ✅ Enter: Sends message
- ✅ Empty input disables send button

---

### Test 8: Error Handling - Network Failure

**Steps:**
1. Stop backend server
2. Try to send a message

**Expected Behavior:**
- ✅ Error message displayed (ElementPlus notification)
- ✅ Message not added to history
- ✅ Input remains enabled
- ✅ User can retry after server restart

---

### Test 9: Mobile Responsiveness

**Steps:**
1. Open browser dev tools
2. Switch to mobile viewport (e.g., iPhone 12)
3. Test all features

**Expected Behavior:**
- ✅ Layout adjusts to mobile screen
- ✅ Session list and chat window remain usable
- ✅ Touch interactions work smoothly

---

### Test 10: Session Persistence

**Steps:**
1. Create a session with 2-3 messages
2. Navigate to another page (e.g., Contacts)
3. Navigate back to Assistant page

**Expected Behavior:**
- ✅ Session list reloads from server
- ✅ Previous session still visible
- ✅ Click to load history successfully

---

## Database Verification

After testing, verify data was persisted correctly:

### Check Session Table
```sql
SELECT * FROM ai_assistant_session ORDER BY created_at DESC LIMIT 5;
```

**Expected Columns:**
- `session_id` (char(20), starts with "AS")
- `tenant_user_id` (char(20), starts with "U")
- `title` (varchar, may be NULL initially)
- `summary` (text, may be NULL initially)
- `agent_id` (char(20), may be NULL)
- `created_at`, `updated_at`

---

### Check Message Table
```sql
SELECT session_id, role, LEFT(content, 50) as preview, 
       JSON_EXTRACT(citations_json, '$[0].score') as first_citation_score,
       JSON_EXTRACT(tokens_json, '$.total_tokens') as total_tokens
FROM ai_assistant_message 
ORDER BY created_at DESC 
LIMIT 10;
```

**Expected Results:**
- ✅ Both `user` and `assistant` roles present
- ✅ `content` field populated
- ✅ `citations_json` is valid JSON array (may be `[]`)
- ✅ `tokens_json` has `prompt_tokens`, `answer_tokens`, `total_tokens`

---

## Performance Benchmarks

### Expected Latencies (with real LLM)

| Metric | Target | Acceptable |
|--------|--------|------------|
| Embedding | < 200ms | < 500ms |
| Vector Search | < 100ms | < 300ms |
| LLM First Token | < 1000ms | < 2000ms |
| LLM Full Response | < 3000ms | < 5000ms |
| Total (End-to-End) | < 4000ms | < 6000ms |

### Load Test (Optional)

Use Apache Bench or similar:
```bash
ab -n 100 -c 10 \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -p request.json \
  http://localhost:8000/ai/assistant/chat
```

**Expected Results:**
- ✅ 0% error rate
- ✅ Mean response time < 5s
- ✅ No memory leaks (monitor with `top` or Task Manager)

---

## Troubleshooting

### Issue: "embedding provider not configured"
**Solution:** 
1. Check `configs/config_local.toml` has valid `[aiConfig.embedding]` settings
2. Restart server after config changes

### Issue: "chat model provider not configured"
**Solution:**
1. Verify `[aiConfig.chatModel]` has valid API key
2. Test API key with provider's test endpoint

### Issue: No citations returned
**Possible Causes:**
1. No RAG data ingested yet (expected if fresh install)
2. Question doesn't match indexed content
3. Milvus connection failed (check logs)

### Issue: SSE stream hangs
**Possible Causes:**
1. LLM API timeout (increase `timeoutSeconds` in config)
2. Network proxy blocking streaming
3. Browser caching (disable in dev tools)

### Issue: Session ID mismatch
**Solution:**
1. Check frontend sends correct `session_id` in subsequent requests
2. Verify JWT `uuid` extraction works (check server logs)

---

## Test Checklist Summary

### Backend ✅
- [ ] Sessions API returns 200
- [ ] Agents API returns 200
- [ ] Chat API creates new session
- [ ] Chat API uses existing session
- [ ] Stream API sends SSE events
- [ ] Error handling works (400, 401, 404)
- [ ] Database records created
- [ ] JWT authentication enforced

### Frontend ✅
- [ ] Assistant page loads
- [ ] New chat button works
- [ ] Message sending works
- [ ] SSE streaming displays incrementally
- [ ] Citations expand/collapse
- [ ] Session switching works
- [ ] Keyboard shortcuts work
- [ ] Error messages display
- [ ] Layout responsive
- [ ] Session persistence works

### Integration ✅
- [ ] Frontend → Backend communication successful
- [ ] SSE stream parsed correctly
- [ ] Citations data matches backend
- [ ] Token counts displayed
- [ ] Session list updates after new message

---

## Next Steps After Testing

1. **If all tests pass:**
   - Mark feature as production-ready
   - Deploy to staging environment
   - Conduct user acceptance testing

2. **If issues found:**
   - Document bugs in GitHub Issues
   - Prioritize by severity
   - Fix and retest

3. **Enhancements (Future):**
   - Add session title auto-generation
   - Implement multi-agent switching
   - Add message editing/regeneration
   - Add export conversation feature
   - Add voice input support

---

**Testing Completed By:** _____________  
**Date:** _____________  
**Status:** ⬜ PASS  ⬜ FAIL  ⬜ PARTIAL  

**Notes:**
```
[Add any observations, bugs found, or recommendations here]
```
