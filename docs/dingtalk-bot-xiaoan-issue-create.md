# 钉钉机器人「小安」在群聊创建 Issue（需求与技术方案）

> 状态：需求细化 + 技术方案（可直接用于实现）  
> 目标：在公司钉钉群聊中 @「小安」，由钉钉机器人回调到 `https://pmo.atuofuture.com/xiaoanmsg`，由 Multica 创建 Issue，并在群里返回创建结果（链接 / 编号 / 失败原因）。

---

## 1. 背景与目标

### 1.1 背景

- 公司已在钉钉开发者平台创建机器人应用「小安」，期望把机器人加入公司群聊。
- 群成员通过 **@小安** 触发创建 Issue，减少打开 Web 的操作成本。
- 计划部署回调地址：`https://pmo.atuofuture.com/xiaoanmsg`（以下称「小安入口」；**尚未实现**，需在服务中新增）。

### 1.2 目标（P0）

**P0 交互策略（已拍板）**：**单轮**——每条群消息独立处理；**任一必填字段缺失或无法匹配**则 **不创建 Issue**，并在群内回复 **固定模板 + 简短原因**（例如「缺少：项目」）。**不做多轮追问**（多轮澄清见 P2）。

- 群聊内任意成员发送包含 **@小安** 的消息时：
  - 小安入口接收钉钉消息事件，完成**验签**（P0 不启用消息体加密）。
  - 解析该条消息（规则解析 + 可选 LLM 辅助抽取），得到 **任务名、项目、责任人（@某人→邮箱→Multica 成员）**。
  - 将 Issue 创建到指定 Multica 工作区与**消息中显式给出的项目**（**每条消息必须包含项目名**，见第 5.4 节）。
  - 将结果回写到群聊（成功：Issue 链接 + 摘要；失败：原因 + **同一套标准模板**；项目名无法匹配时可选附 **LLM 纠错模板**，见第 5.4.4 / 8.6 节）。
- 支持幂等：同一条钉钉消息（`msgId`）重复投递不会创建重复 Issue。

**业务侧必填（小安）**：每条创建指令须能解析出 **任务名（Issue 标题）**、**项目（project）**、**责任人（assignee）**。与 Multica HTTP API 的差异见第 8.4 节。

### 1.3 非目标（首期不做）

- **不做多轮对话**（不保存会话状态、不追问补全）；多轮澄清与流程编排放到 **P2**。
- 不做钉钉登录/绑定授权流程（可用「群成员与邮箱/员工号映射」替代）。
- 不做权限细粒度到项目/字段级别的复杂授权（P0 以工作区级配置与最小权限为主）。

---

## 2. 用户体验与交互（产品需求）

### 2.1 触发方式

- **触发条件**：群消息中 **@了小安**（at 机器人）。
- **输入形式（P0）**：**单条消息内**须包含可解析的 **任务名 + 项目 + 责任人（@某人）**；不支持「先发半句、下一条再补」——下一条会被当作新的独立请求，仍须三要素齐全。

### 2.2 消息模板（建议支持的最小集合）

> **推荐以「键值对」为主**：机器人在群聊里无法像 Web 一样下拉选项目/责任人，**结构化输入**最稳；自然语言可作为 P1 增强（需 NLP 或二次确认）。

#### A. 推荐：一行三要素（最易教、最易解析）

分隔符统一用 **英文分号 `;`** 或 **换行**（二选一写进规范，实现只认一种也可）。

- `@小安 任务=登录页收不到验证码; 项目=PMO 官网; 责任人=@张三`
- 多行写法：

```
@小安
任务=登录页收不到验证码
项目=PMO 官网
责任人=@张三
```

#### A.1 P0 约束（强烈建议写进群公告）

- **必须** @ 小安（触发创建）。
- **必须且只能** @ 1 位责任人（除小安外）。若 @ 多人或未 @，一律失败并返回模板（避免误指派）。

#### B. 同义键名（便于记忆，解析时归一化）

| 含义 | 允许的键名（示例） |
|---|---|
| 任务名 | `任务`、`标题`、`title`、`name` |
| 项目 | `项目`、`project` |
| 责任人 | `责任人`、`指派`、`负责人`、`assignee`（**值建议用 @某人**） |

#### C. 可选扩展（仍放在同一消息里）

- `描述=...` / `备注=...`
- `优先级=P0`（若产品与 Multica 的 priority 枚举对齐后再开放）

### 2.3 返回消息（群内回执）

- **成功**：
  - `已创建 Issue：<标题>（#KEY 或 ID）`
  - `链接：<PUBLIC_APP_URL>/issues/<id>`
  - 可选：状态/优先级/创建人
- **失败（P0）**：
  - 第一行：`创建失败：<原因（可读）>`（例如「缺少：项目」「未找到项目：xxx」「请 @ 一位责任人（除小安外）」）。
  - 第二行起：**固定模板**（与文档 2.2 节 A 一致），便于用户复制修改后重发，例如：

```
请按以下格式一条消息重发：
@小安 任务=<任务名>; 项目=<项目名>; 责任人=@<某人>
（可选）描述=<补充说明>
```

### 2.4 失败与容错（产品侧约定）

- 输入为空或只有 @：提示用法，**不创建**。
- **任务名 / 项目 / 责任人** 任一无法解析或无法匹配到 Multica 实体：**不创建**，群内返回明确错误（例如「未找到项目：xxx」「请 @ 一位责任人」或「未找到成员邮箱：xxx」）+ 标准示例一条。
- 权限/映射缺失：提示管理员需要先完成配置（见第 5 节）。

### 2.5 为何这样定规则（给产品与实现）

1. **三项都是业务必填**：缺少任一项在后续协作里都会产生歧义（进错项目、无人负责），群内又很难补救，因此 P0 采用 **解析失败即拒绝 + 模板重发**（不追问）。
2. **键值对优于纯自然语言**：项目名、人名在中文里变化多，靠分词猜 **项目** vs **任务名** 容易错；键值对解析稳定、可测试。LLM 可用于辅助抽取，但 **不得** 在缺字段时编造项目或责任人。
3. **项目匹配**：Multica 的 `project` 以 **工作区内名称** 为主键展示，建议支持 **精确匹配**；同名项目多个时再要求管理员改名或 P1 支持 `项目ID=<uuid>`。
4. **责任人匹配**：P0 采用 **@某人** 的方式最稳（群聊天然支持、且能拿到钉钉用户标识）。实现上从回调事件解析出被 @ 的用户标识（如 `userId/unionId`），再通过钉钉 API 拉取该用户的 **邮箱**，最终用邮箱匹配 Multica `user.email`（重名问题直接消失）。
5. **项目名（P0 已拍板）**：**每条消息都必须包含 `项目=…`**，不允许仅靠群级配置“默认项目”省略项目名；避免串项目且与产品要求一致。

---

## 3. 总体架构（数据流）

```
钉钉群聊消息（@小安）
  → 钉钉开放平台回调（HTTP POST）
  → 小安入口 /xiaoanmsg
      - 验签（P0 不启用消息体加密）
      - 幂等检查（msgId）
      - 解析消息 → IssueDraft
      - 身份映射（钉钉用户 → Multica 用户/成员）
      - 创建 Issue（调用内部 service 或 REST）
      - 记录投递结果（用于去重与审计）
  → 通过钉钉「发送群消息」API 回写结果（或回调同步响应，视机器人形态）
```

---

## 4. 钉钉侧对接形态与回调验签

> 本项目已确定使用 **企业内部应用机器人（事件回调）**。P0 仅覆盖该路径；其它形态（例如群自定义机器人 Webhook）不在本实现范围。

### 4.1 企业内部应用机器人（事件回调）

- 在钉钉开发者平台配置 **事件订阅/回调地址** 为 `https://pmo.atuofuture.com/xiaoanmsg`。
- 关键点：
  - **验证回调来源**：验签（签名算法）。P0 作为企业内部使用，**不启用消息体加密**（AES 关闭）。
  - **回调响应时限**：必须快速返回（通常 1 秒级），业务处理建议异步化（见第 7 节）。

### 4.3 回调验签（P0 技术要求）

> 具体字段命名以钉钉文档为准；实现以“可配置的验签模块”封装，便于后续需要时再开启加密。

- **验签输入**：以钉钉后台“事件订阅/回调验签”的说明为准（通常包含 `timestamp`、`nonce`、请求体、以及平台配置的 `appSecret`/签名密钥等）。
- **签名算法**：**不要在代码里写死某一种算法**；按钉钉后台给出的算法与拼接规则实现，并对比请求头 `sign`/`signature`。
- **消息体加密**：P0 明确 **不启用**；若未来开启，加密/解密逻辑建议独立成模块，解密失败直接 401/403 不进入业务。

---

## 5. Multica 侧配置与映射（关键需求）

### 5.1 为什么必须映射

创建 Issue 需要明确：

- **落到哪个工作区**（`workspace_id`）
- **由谁发起**（审计：群里谁说的这句话）
- **以谁的身份创建**（`creator` / 权限）
- 可选：默认状态/优先级等（**不包含**省略项目名）

### 5.2 建议的最小配置（P0 必须）

以“群聊”为最小路由单元，配置一条映射：

| 配置项 | 说明 |
|---|---|
| `dingtalk_chat_id` | 钉钉群会话 ID（或群 openConversationId） |
| `workspace_id` | Multica 工作区 |
| `board_id`（可选） | 若将来需要看板级路由可配置（P0 可不填） |
| 默认字段 | 默认 `status`、`priority`、`labels` 等 |
| 允许名单（可选） | 允许创建的人（钉钉 userId 列表 / 部门） |

### 5.3 用户身份映射（P0 两种实现，择一即可）

#### 方案 1：按邮箱映射（推荐、实现简单）

- 责任人：从钉钉回调拿到被 @ 的用户标识（如 `userId/unionId`），通过钉钉 API 拉取该用户 profile，拿到 **邮箱**。
- 发起者：同理可用发送者的用户标识拉取邮箱，用于在 Multica 侧找到发起者（审计/权限校验）。
- 使用 `email` 在 Multica `user.email` 中查找发起者与责任人。
- 找不到则：
  - 方案 A：创建失败，提示“未开通 Multica 账号”；
  - 方案 B：自动创建用户（若 Multica 支持“首次登录自动创建”，需要审慎）。

##### 创建者身份（P0 已拍板）

- **Issue 的 creator 为消息发送者（发起者）**，即 **谁发的这条钉钉消息，谁就是 Multica 侧 Issue 的创建者**（对应 `user` / member 身份），**不是**机器人账号。
- 实现路径：发送者钉钉用户标识 → 拉取 **邮箱** → 在 Multica 中解析为 `user_id` → 作为创建 Issue 时的 **creator**。
- `/xiaoanmsg` 为外部回调，通常没有浏览器里的 Multica JWT；应在 **钉钉验签通过后**，由服务端走 **受信任内部路径** 创建 Issue（内部 service / 仅服务端可调用的 handler），**仅允许**将 `creator_id` 设为已解析到的发起者 `user_id`，**禁止**接受客户端随意指定任意用户 ID。
- 同时强校验：发起者必须是该 workspace 成员，否则拒绝创建。

#### 方案 2：按员工工号/unionId 映射（更稳定）

- 增加一张映射表（概念）：`user_identity(provider="dingtalk", union_id, user_id, workspace_id)`。
- 管理员通过一次性导入/绑定建立映射。

### 5.4 项目解析策略（P0 必须写死，避免“进错项目”）

P0 推荐 **仅允许“确定性解析”**：能明确落到唯一 `project_id` 才创建，否则失败返回模板。

#### 5.4.1 解析优先级（P0）

1. **消息中必须显式提供项目**（`项目=...` / `project=...`）：按该值解析；**未写 `项目=` 一律失败**：`缺少：项目`。
2. **不允许**用群级「默认 `project_id`」代替用户在消息里写项目名（与产品要求一致）。

#### 5.4.2 匹配规则（推荐）

- **精确匹配优先**：将用户输入 `项目=` 的值与工作区内 `project.name` 做 **去首尾空格 + 大小写不敏感** 的精确匹配。
- **别名表（强烈建议）**：支持配置 `alias → project_id`（例如「官网」「PMO官网」都指向同一项目）。P0 若不做别名，务必要求用户填写项目全名。
- **禁止模糊匹配作为默认**：避免“看起来差不多”但落错项目。

#### 5.4.3 重名/多候选处理（P0）

- 若存在多个候选：P0 直接失败并提示“项目不唯一，请填写全名或管理员配置别名”，并附模板。

#### 5.4.4 项目名写错 / 无法匹配（P0）

- 当 `项目=` 的值在工作区内 **匹配不到唯一 `project_id`** 时：**不创建**，返回 `project_not_found` / `project_ambiguous`（见第 7.4 节）。
- **可选增强（建议启用，配合 LLM）**：在失败回执中，除固定模板外，追加 **一条「纠错建议」**，格式仍为键值对模板，便于用户 **整段复制后微调再发**，例如：
  - 根据当前 workspace 的 **项目列表**（名称来自数据库，非杜撰），让 LLM 生成与用户输入最接近的 **候选 `项目=…` 写法**（1～3 个候选即可）；
  - 明确提示：**候选仅供参考**，最终以管理员在 Multica 中的项目名为准。
- **硬约束**：LLM **不得** 编造不存在的项目名；候选必须来自 **该 workspace 已有 `project.name`（或别名表）**。

---

## 6. 小安入口 `/xiaoanmsg` 的接口契约

### 6.1 入站：钉钉 → 小安入口

- **Method**：`POST`
- **Path**：`/xiaoanmsg`
- **Headers**（示例，按钉钉实际为准）：
  - `timestamp`
  - `nonce`
  - `signature` / `sign`
  - `content-type: application/json`
- **Body**：
  - P0 明文 JSON（企业内部使用，不启用加密）
  - 关键字段需求（概念）：
    - `msgId`：消息唯一 ID（用于幂等）
    - `conversationId`：群 ID（用于路由到 workspace）
    - `senderId` / `senderUnionId` / `senderStaffId`：发信人标识（用于身份映射）
    - `text.content`：消息内容
    - `atUsers`：被 @ 的用户列表（用于确认 @ 了小安，并找到被 @ 的责任人）

> P0 约束：`atUsers` 中 **必须包含小安**，且除小安外 **必须且只能包含 1 位责任人**。若 @ 多人或未 @ 责任人，按 `missing_assignee` 失败并回固定模板。

### 6.2 出站：小安 → 钉钉群回写

两种可选方式（由机器人形态决定）：

#### A. 回调同步响应（若钉钉支持机器人回调直接回复）

- HTTP 200，body 返回 `responseContent`（或平台要求的结构）。
- 缺点：创建 Issue 可能超时；建议仅用于“已接收”提示，结果用异步消息再发一次。

#### B. 调用钉钉「发送群消息」API（推荐）

- 小安入口立即 `200 OK`；
- 后台异步创建 Issue；
- 创建完成后调用钉钉 API 发消息到群：
  - 成功/失败都回写；
  - 可 @ 原发送人（增强可见性）。

##### P0 约定：统一采用“发送群消息 API”回写最终结果

为避免回调超时与重复投递导致体验不一致，P0 建议：

- 入站回调只做“接收成功/失败”的 HTTP 返回（满足平台时限）；
- 最终结果（成功/失败模板）**一律**通过钉钉发群消息 API 回写。

##### AccessToken 获取与缓存（实现要点）

- 使用 `appKey/appSecret` 换取 AccessToken（按钉钉官方流程）。
- 在服务端缓存 token 与过期时间（内存即可，进程重启自动重取）。
- 发群消息与“拉用户邮箱”的 API 共用该 token。

### 6.3 内部标准化事件结构（适配层，P0 推荐）

由于钉钉回调字段名较多，建议在 `/xiaoanmsg` 的最前面做一层 **适配**，将入站事件规范化为内部结构（后续业务逻辑只依赖该结构）：

- `provider`: `"dingtalk"`
- `msgId`: string（幂等键）
- `conversationId`: string（群路由键）
- `sender`:
  - `userId` / `unionId`（至少一个）
  - `displayName`（用于群回执可读）
- `at`:
  - `botMentioned`: boolean（是否 @ 了小安）
  - `mentionedUserIds`: string[]（被 @ 的用户标识列表，不含小安）
- `text`:
  - `raw`: string（原始文本）
  - `normalized`: string（去掉 @小安 的文本；保留其余内容）
- `raw`: object（原始 payload，落日志/排障用；不要回写到群里）

### 6.4 P0 业务处理伪代码（到可直接编码）

```
handleInbound(request):
  assert verifySignature(headers, rawBody) == true
  event = normalizeDingTalkEvent(headers, rawBody)
  if !event.at.botMentioned: return 200

  if deliveryExists(event.msgId): return 200   // 幂等：钉钉重试不重复创建
  saveDelivery(event.msgId, status="received", conversationId=event.conversationId)

  enqueueJob(event.msgId, event)
  return 200

worker(job):
  event = job.event

  // 责任人：必须且只能 @ 1 人（不含小安）
  if len(event.at.mentionedUserIds) != 1:
    fail("missing_assignee")

  assigneeDingId = event.at.mentionedUserIds[0]
  assigneeEmail = dingtalkGetUserEmail(assigneeDingId)
  initiatorEmail = dingtalkGetUserEmail(event.sender.userId|unionId)

  workspace = resolveWorkspaceByConversationId(event.conversationId)
  assert initiatorEmail is member of workspace

  // 解析项目：消息内必须有 项目=...，且能唯一匹配 project_id（否则失败，可附 LLM 候选）
  projectId = resolveProjectIdRequired(workspace, kvPairs)

  // 解析标题：优先 任务=...；否则在 项目= / 责任人=@ 已齐的前提下用剩余正文
  title = extractTitle(event.text.normalized, kvPairs, llmOptional)

  creatorUserId = resolveUserIdByEmail(initiatorEmail)

  // Multica 创建：creator=发起者；assignee=被 @ 的责任人（均为 member）
  issue = multicaCreateIssueTrusted({
    creatorUserId,
    title, project_id: projectId,
    assignee_type:"member", assignee_id: resolveMemberIdByEmail(assigneeEmail),
    description: descriptionWithAuditFooter(...)
  })

  saveDeliveryCreated(event.msgId, issue.id)
  dingtalkSendGroupMessage(successMessage(issue))
catch classifiedError e:
  saveDeliveryFailed(event.msgId, e.code, e.message)
  dingtalkSendGroupMessage(failureTemplate(e))
```

---

## 7. 幂等、重试与异步化（工程要求）

### 7.1 幂等

- 使用 `msgId` 作为幂等键（同一消息只创建一次）。
- 建议新增投递记录表（概念）：
  - `dingtalk_message_id (unique)`
  - `workspace_id`
  - `issue_id (nullable)`
  - `status`（received/created/failed）
  - `error`、`created_at`

### 7.2 异步处理

为满足钉钉回调时限与稳定性：

- 回调线程只做：验签、最小校验、落库「已接收」、入队（P0 无解密）。
- 后台 worker 做：身份映射、解析、创建 Issue、回写钉钉消息、更新投递记录。

### 7.3 重试策略

- **入站回调**：钉钉可能重试投递；依赖幂等表避免重复创建。
- **出站发消息到钉钉**：
  - 网络/5xx：指数退避重试；
  - 4xx（参数/权限）：标记失败并告警；
  - 最终失败：在 Multica 日志中保留可追踪信息（msgId、conversationId）。

### 7.4 错误码与群回执映射（P0 必须统一）

建议实现一个“可测试的错误分类”，并把每类错误映射到固定的群回执（避免散落在代码里各写各的）。

| 错误类型 | 触发条件（示例） | 群内第一行（原因） |
|---|---|---|
| `missing_title` | 无法确定任务名 | `创建失败：缺少：任务` |
| `missing_project` | 未写 `项目=` | `创建失败：缺少：项目` |
| `project_not_found` | 项目名解析不到唯一 project | `创建失败：未找到项目：<name>`（可选第三段起：LLM 候选纠错模板，见第 5.4.4 节） |
| `project_ambiguous` | 项目匹配到多个候选 | `创建失败：项目不唯一：<name>` |
| `missing_assignee` | 未 @ 责任人或 @ 多人 | `创建失败：请只 @ 1 位责任人（除小安外）` |
| `assignee_email_missing` | 钉钉侧拉不到邮箱 | `创建失败：无法获取责任人邮箱（请检查钉钉通讯录权限）` |
| `assignee_not_in_multica` | 邮箱不在 Multica 用户表 | `创建失败：责任人未开通 Multica 账号：<email>` |
| `initiator_not_in_workspace` | 发起者不是 workspace 成员 | `创建失败：你不在该工作区，无法创建` |
| `permission_denied` | 发起者无创建权限 / 内部调用失败 / Multica 403 | `创建失败：权限不足（请联系管理员）` |
| `system_error` | 其它异常 | `创建失败：系统错误，请稍后重试` |

所有失败回执的第二段必须附上文档 2.3 的 **固定模板**。

---

## 8. 消息解析与 Issue 字段映射

### 8.1 解析输出模型（概念）

`IssueDraft`（小安 P0）：

- `title`（任务名，必填）
- `project_id`（解析路径：必须由消息中的 **`项目=...`** 得到 **项目名** → 在工作区内解析为 UUID，**必填**）
- `assignee_type` + `assignee_id`（责任人，必填；一般为 `member`，少数场景为 `agent`）
- `description`（可选；未提供时可为空，或填入用户 `描述=`）
- `priority`、`dueDate`、`labels`（可选，P1）

### 8.2 任务名（title）

- 优先来自 `任务=` / `标题=` 等同义键；若用户未写 `任务=`，但同一条消息内已 **显式包含 `项目=...` 且恰好 @ 1 位责任人**，可将「去除 @小安、去掉 `项目=…` 键值对、去掉责任人 @」后的剩余文本作为 `title`（仍视为单轮且可验证）。
- **禁止**在业务三要素不全时用「截取正文」凑标题，以免误创建。

### 8.3 描述内容（建议）

若用户提供了 `描述=` / `备注=`，写入 Issue `description`；否则可为空。下列元数据可追加在描述末尾（便于审计）：

- 原始消息全文
- 钉钉群信息（群名若可获取）
- 发送人（钉钉显示名 + 标识）
- 创建时间

### 8.4 与 Multica `POST /api/issues` 的差异（实现时注意）

Multica 后端创建 Issue 时（概念）：

- **`title`**：必填。
- **`project_id`**：可选；未传时，服务端会选用 **工作区内第一个 project**（若工作区无任何 project 则报错）。
- **`assignee_type` / `assignee_id`**：可选；未传则无指派人。

**小安业务**要求群内指令必须凑齐 **项目 + 责任人 + 任务名**：应在 **机器人服务层** 校验；通过后再调用 Multica（**creator=发起者 user_id**，**assignee** 指派人，**project_id** 来自 `项目=`）。不得依赖 HTTP API 对 `project_id` 的“默认第一个项目”行为。

### 8.5 LLM 辅助抽取（P0 可选，但建议先留接口）

> P0 已确定“缺字段就失败”，所以 LLM 在 P0 的价值主要是：从自然语言里 **抽取 title / projectName**（而不是编造）。责任人以 @ 为准。

建议让 LLM 输出严格 JSON（解析失败视为失败并返回固定模板，或回退到纯规则解析）：

- `title`: string | null
- `project_name`: string | null
- `description`: string | null
- `missing_fields`: string[]（仅用于诊断；最终仍以服务端解析与可验证匹配为准）

硬约束（必须写进 prompt / system rules）：

1. `project_name` 必须来自用户原文中出现的文本，不得凭空新增。
2. 不得生成责任人信息（责任人来自 @ 用户）。

### 8.6 LLM 项目名纠错（仅失败回执，P0 可选）

- 触发条件：`project_not_found` 或 `project_ambiguous`。
- 输入：用户填写的 `项目=` 原始字符串 + 当前 workspace 下 **项目名列表**（来自数据库）。
- 输出：1～3 条 **完整可复制** 的一行模板（含 `@小安`、`任务=`、`项目=`、`责任人=@` 占位），**项目名必须来自列表**。
- 不用于自动创建 Issue，仅用于降低用户重试成本。

---

## 9. 安全与权限（必须明确）

### 9.1 外部入口安全

- **仅允许钉钉回调**：强制验签；验签失败直接拒绝。
- **IP 白名单（可选）**：若钉钉提供固定网段，可作为第二道防线。
- **限流**：按 `conversationId` / IP 做速率限制，防止误配/刷接口。

### 9.2 工作区权限

- 群 → workspace 的映射由管理员配置。
- **发起者**必须是该 workspace 成员；否则拒绝并提示“未加入工作区/未开通账号”。
- **creator=发起者**：发起者须具备在该 workspace 创建 Issue 的权限（与 Multica 现有成员权限模型一致）。

### 9.3 数据最小化

- 回写群里的内容避免泄露敏感信息：
  - 成功回写只包含标题 + 链接；
  - 详细描述留在 Issue 内。

---

## 10. 运维与配置清单（落地步骤）

### 10.1 钉钉平台配置

- 创建/发布机器人应用「小安」。
- 配置事件订阅/机器人回调 URL：`https://pmo.atuofuture.com/xiaoanmsg`
- 获取并安全保存：
  - `appKey` / `appSecret`（或签名密钥）
- （P0 不启用加密，无需 `aesKey`）
- 配置机器人权限：读取用户信息（若需要 email）、发送群消息等。

### 10.2 Multica 服务端配置（建议以环境变量 + DB 配置结合）

- 环境变量（示例）：
  - `DINGTALK_XIAOAN_APP_KEY`
  - `DINGTALK_XIAOAN_APP_SECRET`
  - `PUBLIC_APP_URL`（生成 Issue 链接）
  - `XIAOAN_LLM_PROVIDER` / `XIAOAN_LLM_MODEL` / `XIAOAN_LLM_BASE_URL` / `XIAOAN_LLM_API_KEY`（可选）
- 数据库配置：
  - 群（conversationId）→ workspace 路由表
  - 幂等投递表
  - 可选：用户身份映射表

### 10.3 联调验收用例（P0）

- **创建成功**：`@小安 任务=…; 项目=…; 责任人=@…` → 生成 Issue → 群回写链接可打开。
- **三要素缺一**：不创建，群内提示原因 + **固定模板**（可复制重发）。
- **分两条消息补信息**：第二条不合并上一条上下文（P0 无会话）；仍须单条含三要素。
- **项目名不存在 / 成员无法匹配**：不创建，错误信息可读。
- **幂等**：同一 msgId 重放 → 不重复创建。
- **未配置群映射**：提示管理员配置。
- **未映射用户**：提示用户开通/管理员绑定。
- **验签失败**：返回 401/403，且不产生任何副作用。

### 10.4 数据表/持久化建议（P0）

P0 至少需要两类持久化（不要求一定做成这些表名，但要有等价能力）：

1. **群路由表**：`conversationId → workspace_id (+ allowlist...)`（**不需要**默认 `project_id` / bot 用户）
2. **投递幂等表**：`msgId` 唯一键，记录 received/created/failed、`issue_id`、错误信息与时间戳（用于排障与避免重复创建）

---

## 11. 里程碑拆分（建议）

| 阶段 | 交付物 |
|---|---|
| **P0** | 回调验签（不加密）+ 群路由配置 + **单轮**解析 + 缺字段失败并返回固定模板 + msgId 幂等 + 异步创建 Issue + 群回写结果 |
| **P1** | 字段解析增强（priority/due/labels）+ 允许名单 + 告警/监控面板 |
| **P2** | 多轮对话澄清、模板化表单交互、与 Agent 自动分派联动 |

---

## 12. 已确认结论与剩余问题

### 12.1 已确认

1. **钉钉形态**：使用 **企业内部应用机器人（事件回调）**；回调入口为 `https://pmo.atuofuture.com/xiaoanmsg`（实现时按钉钉开放平台「事件订阅 / 机器人」文档接入验签与消息体字段；**P0 不启用加密**）。
2. **P0 交互**：**单轮**；缺字段或匹配失败则 **不创建** 并返回 **固定模板**（见第 2.3 节）。
3. **入口实现状态**：Multica 服务端已实现 **`POST /xiaoanmsg`**（见第 13 节）；生产需完成迁移、环境变量与群映射配置。
4. **业务必填字段**：群内创建 Issue 须提供 **项目（project）**、**责任人（assignee）**、**任务名（issue 标题）**；**`项目=` 每条消息都必须出现**（不允许用群级默认项目省略）。与 Multica API 默认行为的关系见 **第 8.4 节**。
5. **创建者**：**creator = 消息发送者**（发起者），见第 5.3 节「创建者身份」。

### 12.2 仍待产品/运维拍板（可选）

1. **项目别名表**（`alias → project_id`）是否首期上线：可显著减少「项目名写错」。
2. **责任人**解析：**@某人 → 钉钉 API 取邮箱 → 匹配 Multica `user.email`**（与当前方案一致）。

---

## 13. 代码实现说明（本仓库）

### 13.1 路由

| 路径 | 说明 |
|---|---|
| `POST /xiaoanmsg` | 钉钉回调入口；校验签名后异步处理；HTTP 立即返回 `{"success":true}` |
| `POST /api/internal/issues/as-user` | 服务端代创建 Issue（`creator_email` = 发起者）；请求头 `X-Internal-Secret` = `INTERNAL_API_SECRET`（供其它内部服务调用；小安主流程直接调 handler 核心逻辑，不必自调用此 HTTP） |

### 13.2 数据库

- 迁移：`server/migrations/034_dingtalk_xiaoan.up.sql`（`dingtalk_xiaoan_chat_mapping`、`dingtalk_xiaoan_delivery`）
- sqlc：`server/pkg/db/queries/dingtalk_xiaoan.sql`（含 `UpsertDingtalkXiaoanChatMapping`，便于运维写脚本或后续接管理接口）

**群映射示例（SQL）**（将 `conversation_id` 换成钉钉回调里的群会话 ID，`workspace_id` 换成 Multica 工作区 UUID）：

```sql
INSERT INTO dingtalk_xiaoan_chat_mapping (conversation_id, workspace_id)
VALUES ('<dingtalk_conversation_id>', '<multica_workspace_uuid>')
ON CONFLICT (conversation_id) DO UPDATE SET workspace_id = EXCLUDED.workspace_id;
```

### 13.3 环境变量（参见根目录 `.env.example`）

| 变量 | 说明 |
|---|---|
| `DINGTALK_XIAOAN_APP_KEY` / `DINGTALK_XIAOAN_APP_SECRET` | 换 `access_token`、验签 |
| `DINGTALK_XIAOAN_SKIP_VERIFY` | 设为 `1` 时**跳过**回调验签（仅本地调试） |
| `DINGTALK_XIAOAN_BOT_USER_IDS` | 逗号分隔，机器人对应的钉钉 **userId**（必须在 `atUsers` 中出现才会处理；并用于从 @ 列表中剔除机器人） |
| `DINGTALK_XIAOAN_ROBOT_CODE` | 开发者后台机器人 **robotCode**，用于 `api.dingtalk.com/v1.0/robot/groupMessages/send` 回执 |
| `MULTICA_APP_URL` | 成功消息里的 Issue 链接前缀 |
| `INTERNAL_API_SECRET` | 保护 `/api/internal/*` |

### 13.4 验签与消息体

- 当前实现：`Base64(HMAC-SHA256(appSecret, timestamp + "\n" + nonce + "\n" + body))` 与 Query/Header 中的 `sign` / `signature` 比对（若与钉钉控制台不一致，以官方文档为准并改 `internal/dingtalk/signature.go`）。
- 若回调 Body 顶层含 **`encrypt`**：本实现返回 `encrypted_callback_not_supported_in_p0`（套件 AES 回调需另接解密逻辑）。

### 13.5 钉钉 OpenAPI 调用

- 取用户邮箱：`topapi/v2/user/get`（需通讯录权限；`userid` 来自回调里的 `senderStaffId` / `atUsers` 等字段，以实际回调为准）。
- 群回执：`POST https://api.dingtalk.com/v1.0/robot/groupMessages/send`（`sampleText`）。

### 13.6 Nginx（chandao / `pmo.atuofuture.com`）

默认站点配置里 **`/` 会反代到 Next.js**，**必须**把 `POST /xiaoanmsg` 反代到 Go（`127.0.0.1:8080`）。仓库已包含片段：`scripts/deploy/nginx-multica.conf` 中 `location = /xiaoanmsg { ... }`。

服务器上在合并该配置后执行：`sudo nginx -t && sudo systemctl reload nginx`。

