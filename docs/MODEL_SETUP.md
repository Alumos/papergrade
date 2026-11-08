# OCR / OpenAI 协议大模型：实际接入步骤

> v0.2 已支持 **Ollama原生图像OCR**：后台选类型、填基址与模型、密钥可空；PDF由浏览器按页转换。文本分析用OpenAI-compatible Chat Completions。新学号标准卡内置浏览器OMR+Go复核，无需CARD。旧网关兼容，腾讯云等厂商SDK仍未内置。见 [v0.2操作说明](<RELEASE_V02.md>)。

## 1. 三项服务分别做什么

| 服务 | 用途 | 当前可直接填什么 |
|---|---|---|
| AI 文本大模型 | 把 OCR 文本整理为题号、题型、分值、答案候选 | 支持 Chat Completions 和 JSON 输出的 OpenAI-compatible 基址、密钥、模型名 |
| 原卷 OCR | PDF页图片／图片 → 文字和LaTeX | Ollama原生基址+模型，或兼容自定义网关 |
| 新答题卡识别 | 单卡图 → 学号+每题选项 | a5-v2内置OMR，无需模型或CARD URL；二维码仅考试ID |

上述 AI 配置不会自动开启 PDF 视觉识别。当前系统不把 PDF 当文本发给 /chat/completions，也没有把答题卡图送给文本模型猜填涂。模型能读图不等于当前适配器已支持其多模态协议。

## 2. 后台配置（推荐，不用反复改部署文件）

登录系统 → 顶部 **设置** → **模型与服务设置** → 分别选择 OCR、AI 模型、答题卡网关。当前单教师部署中，唯一教师账号兼任管理员；尚不是多角色学校管理系统。

1. 输入 OCR / 答题卡网关完整 URL，或 AI 的 API Base URL 与模型名。
2. API Key 填在独立密码框。已有密钥仅显示“已配置”，不会从服务端回传。
3. 空密钥表示保留原密钥；只有勾选“清除已有密钥”才会删除。填写新密钥和清除同时选择会被拒绝。
4. 点击保存并确认。配置写入 PostgreSQL，带 revision 并发检查，版本过期返回409且保留本地输入；先重新读取再核对，不自动覆盖别人修改。
5. 保存成功后 API 和 Worker **下一次云端调用读取最新配置**，不需重启容器、重新打包或修改部署文件。已经发出的请求使用原配置继续执行，不强行中断。
6. 点击“测试已保存配置”显式检查地址/鉴权。AI 使用 GET /models，OCR / 卡网关使用 HEAD，限时8秒，不上传试卷、不请求模型推理。405代表接口可达但不支持此探测，不等于识别失败。通过也不证明模型ID、JSON推理协议和识别准确率已验收。

默认URL只允许HTTPS；Ollama额外允许私有LAN／CGNAT literal IP的HTTP，loopback测试也可HTTP；拒绝URL中内嵌用户密码、query和fragment，密钥必须放独立字段。不要仅为消除“未配置”提示而填一个未适配地址。

### 首次部署仍需一个应用加密主密钥

模型地址、模型名和供应商密钥不必在部署时设置。但为了安全保存它们，基础设施首次部署需要配置 **APP_ENCRYPTION_KEY**（不是模型API Key）：

~~~sh
openssl rand -hex 32
~~~

将生成值只写入部署机的私有 .env 中 APP_ENCRYPTION_KEY，确保 api 和 worker 使用同一个稳定值。它至少32字节，服务端派生AES-256-GCM密钥，以随机nonce与AAD加密整份配置；数据库与审计不写明文供应商密钥。不要把真实主密钥写入仓库、镜像或公开日志。

**主密钥必须单独备份，重启和升级不可随意更换。** 丢失或更换后既有配置无法解密，系统明确503而不是悄悄使用旧环境变量。当前未实现自动主密钥轮换；应在保有旧密钥时规划迁移，不能靠重新生成替代。备份数据库时同时安全保存该密钥，但不要放在同一份未加密公开归档。

已有部署升级时，只需一次加入主密钥并更新版本/重建api和worker。以后日常模型设置全在后台完成。

~~~sh
# 初次升级到后台模型设置版本，先暂停扫描并等在途任务完成
# 先在 .env 配置稳定 APP_ENCRYPTION_KEY，及正确 IMAGE、域名、账号等基础设施
# 模型相关变量可以留空
docker compose pull
docker compose run --rm api migrate
docker compose up -d --no-build --force-recreate api worker gateway
~~~

DEV_MODE + 内存开发模式无主密钥时使用临时随机密钥并醒目警告：重启丢配置，不能用于正式考试。只要使用PostgreSQL，开发模式也必须配置稳定主密钥；生产无主密钥会拒绝启动。

### 兼容旧部署的环境引导

OCR_ENDPOINT/OCR_API_KEY、AI_BASE_URL/AI_API_KEY/AI_MODEL、CARD_ENDPOINT/CARD_API_KEY 保留为**首次后台保存之前**的可选引导配置。后台保存过一次后，整份数据库配置优先；清空某一服务表示禁用，不会再用环境值把它重新启用。界面显示来源为“后台配置”或“环境引导配置”。

不需要在 GitHub Actions Secrets 中保存模型密钥，CI 构建与测试使用一次性测试主密钥，不调用你的真实模型。

## 3. AI 接口准确契约

系统请求：后台 AI Base URL（旧部署对应 AI_BASE_URL）去掉末尾 / 后，再拼接 /chat/completions。例如基址 https://provider.example/v1 会请求 https://provider.example/v1/chat/completions。

- Authorization: Bearer AI_API_KEY
- 请求体包括 model、messages、temperature:0、max_tokens:6000、response_format:{type:json_object}。
- 非流式响应，读取 choices[0].message.content 内的 JSON 字符串。
- 供应商须支持这些参数和 JSON object 输出；只支持 Responses API、特殊签名、不同参数名或不同响应结构的服务，需额外适配。不能直接填 /responses。
- 默认90秒超时、输入文本≤120KB、HTTP响应≤2MB；最多40道候选题。
- 不会自动根据模型推理补出原卷没有的标准答案。缺答案 []、缺分值 null，教师补齐确认后才发布。

返回内容形状示例（这是 message.content 解码后的对象）：

~~~json
{
  "questions": [{
    "number": "1",
    "type": "single_choice",
    "options": ["A", "B", "C", "D"],
    "answer": [],
    "score_units": 200,
    "source_page": 1,
    "review_flags": ["missing_answer"]
  }],
  "warnings": ["原卷未给出标准答案，请教师填写"]
}
~~~

100 score_units = 1分。云端输出只作为候选，不直接生成正式分数。

## 4. Ollama原生与旧网关

### Ollama原生（推荐）
后台OCR类型选Ollama，地址填自己的HTTPS DDNS基址，不加/api/chat；模型名填NAS实际完整名字。默认地址为空，不预置NAS IP。原生Ollama通常没有API密钥，可以留空；反代增加Bearer才填。PDF在浏览器点击准备页面，逐页上传后解析；失败显式重试复用已完成页，页面显示Markdown与LaTeX。

### 自定义网关（兼容）


当前原卷OCR网关请求为：POST OCR_ENDPOINT，Content-Type: application/pdf，请求体为 PDF 二进制，Bearer OCR_API_KEY（若设置）。返回：

~~~json
{"text":"识别出的试卷文字，保留题号、选项和分值","pages":3}
~~~

这不是阿里云、腾讯云、百度、PaddleOCR、Mistral 或其他厂商接口都天然支持的标准。若其要求签名、base64、文件URL、上传后异步轮询、自定义JSON、OpenAI视觉消息，则要在 Go 内增加对应 provider 适配器，把其结果映射为 text/pages。**目前没有一个已部署的通用OCR网关可以让你直接填上就用。**

不建议为此在2C4G机器再部署Python模型；优先在Go中做轻量HTTP适配，计算仍在云端。

## 5. 连续阅卡：新学号卡内置OMR

新发布a5-v2，A4上下双联裁开后单卡扫描：浏览器Web Worker预读学号/选项，Go独立复核计分，不调用OCR／AI／CARD。学号数字等宽并保留前导零；二维码全班同考试ID，不校验学生。填错另一合法学号无法凭二维码发现，需教师对账。

以下仅适用于旧a4-v1／第三方识别网关兼容路径：


CARD_ENDPOINT 接收 multipart image + template，响应 token、每题 selected / state / confidence，详情见云端接口契约。

- 不信任浏览器识别的学生绑定，网关须独立读码，服务端再检查该码属于已发布考试。
- 普通文字OCR可能只识别出题号和标签，不能可靠识别填涂、擦除、多选。
- OpenAI-compatible多模态模型需要额外图像请求适配和质量评估，当前文本AI配置不会自动接管扫卡。
- 没配CARD_ENDPOINT时只能建卷 / 打印，自动扫卡会明确返回503；不能称端到端正式阅卷已经可用。
- 发送学生答题卡会涉及姓名/学号、个人码与作答信息，选供应商前确认授权、保存/训练/地域政策。

## 6. 配好以后怎么操作

1. 教师登录，确认页面AI/OCR状态。注意此处目前仅检查配置是否非空，页面加载不自动探测，点击测试按钮才检查地址/鉴权；**“已配置”不等于连通测试或识别成功**。
2. 创建班级 / 名册，再创建考试草稿。
3. 在“PDF与解析”上传原卷，选择文件，点击解析。原卷OCR与AI两者都配好才会受理解析任务。
4. worker调用OCR，再调用AI；页面轮询 uploaded → queued → processing → parsed / failed。
5. 查看候选，显式采纳到草稿（已有题目需要确认覆盖），检查题型、数量、分值与答案。缺值必须手工补，不允许猜。
6. 发布考试、打印个人卡。CARD网关接入并验证后才能正式连续识别入库。

只配AI而没配OCR不会完成PDF解析；没有标准答案也不会自动获得可靠答案。

## 7. 常见错误

| 现象 | 检查 |
|---|---|
| PROVIDER_NOT_CONFIGURED / 503 | OCR+AI解析缺一个，或扫卡缺CARD；确认后台保存成功，API/Worker连接同一数据库且主密钥一致 |
| 页面显示已配置但parsed失败 | 非空状态不验证鉴权，检查URL、密钥权限、协议与模型参数 |
| AI 401 / 403 | key、账号余额/权限、模型访问范围、网关鉴权方式 |
| AI 404 | 不要在AI_BASE_URL里重复/chat/completions，核对/v1和模型ID |
| AI 400 | 供应商是否支持response_format/json_object、temperature、max_tokens |
| OCR失败 | 当前是raw PDF协议，不是厂商原生JSON/签名接口；必须适配 |
| PDF一直queued | worker未运行、数据库不同、队列失败，查看api/worker日志 |
| CARD已配置但识别失败 | 协议不符、码未读出、题ID/选项/状态不完整、不属于当前考试 |

应用错误会脱敏，不把供应商密钥/完整错误响应写到教师界面。后台已提供显式地址/鉴权测试，但不执行推理或上传真实文件；详细调用看板、厂商原生适配和真实推理验收仍需继续补齐。

## 8. 下一步需要你提供的非密钥信息

- 原卷OCR供应商/模型名、API文档链接，或一份脱敏请求/响应示例。
- 大模型的Base URL、模型ID，以及是否支持Chat Completions JSON object；**不需要把真实密钥发到聊天或提交仓库**。
- 答题卡打算用专门OMR服务，还是某个云端视觉模型；对应接口文档和隐私要求。

拿到这些信息后才能做厂商原生直连适配和真实连通验收，而不只是让你配置一个尚不存在的网关。
