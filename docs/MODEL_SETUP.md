# OCR / OpenAI 协议大模型：实际接入步骤

> 先讲清现状：文本大模型已实现 OpenAI-compatible Chat Completions 直连；原卷 OCR 和答题卡识别目前实现的是通用网关协议，**尚未内置某家 OCR 厂商原生 SDK**。不需要在 VPS 自部署模型。不能把“已配置”当成“已连通”或“识别已验收”。

## 1. 三项服务分别做什么

| 服务 | 用途 | 当前可直接填什么 |
|---|---|---|
| AI 文本大模型 | 把 OCR 文本整理为题号、题型、分值、答案候选 | 支持 Chat Completions 和 JSON 输出的 OpenAI-compatible 基址、密钥、模型名 |
| 原卷 OCR | PDF → 文字，保留题号、分值与页码 | 实现本系统原始 PDF 请求 / text+pages 响应的网关；普通厂商接口需适配 |
| 答题卡识别 | 卡片图 → 个人卡 token + 每题填涂结果 | 实现 card-v1 的专用识别网关；不是仅凭普通文字 OCR 就能完成 |

上述 AI 配置不会自动开启 PDF 视觉识别。当前系统不把 PDF 当文本发给 /chat/completions，也没有把答题卡图送给文本模型猜填涂。模型能读图不等于当前适配器已支持其多模态协议。

## 2. 配置放在哪里

当前从部署机器的 .env 读取，由 Compose 传入 api 和 worker。**不是在前端填写，也不是 GitHub 仓库 Secrets**；仓库不需要保存你真实的模型密钥，Actions 构建镜像也不需要模型密钥。

当前教师页面仅显示能力状态，没有实现网页后台修改供应商地址 / 密钥的设置页。

在仓库根复制 .env.example 为 .env 后，编辑：

~~~dotenv
# 这里给的是格式样例。替换为你的域名及账号，不能照抄示例密钥。
DOMAIN=grade.your-domain.example
ADMIN_USERNAME=teacher
ADMIN_PASSWORD=replace-with-your-own-long-random-password
POSTGRES_PASSWORD=replace-with-your-own-random-hex-password

# OpenAI-compatible文本模型：填写“基址”，不包含/chat/completions
AI_BASE_URL=https://your-model-provider.example/v1
AI_API_KEY=replace-with-your-model-api-key
AI_MODEL=replace-with-the-exact-model-id

# 原卷OCR：仅在接好对应网关后填写完整接口URL
OCR_ENDPOINT=https://your-ocr-gateway.example/extract
OCR_API_KEY=replace-with-your-ocr-gateway-key

# 学生卡：专用card-v1网关，与原卷OCR分开
CARD_ENDPOINT=https://your-card-gateway.example/recognize
CARD_API_KEY=replace-with-your-card-gateway-key

LOCAL_OMR_ENABLED=false
~~~

不同服务可用不同密钥。不要把空的或未适配的 endpoint 用随便一个地址填满以消除“未配置”提示。所有外部端点必须 HTTPS，HTTP 仅允许本机 loopback 测试。

如果使用 OpenAI 官方服务，AI_BASE_URL 的格式是 https://api.openai.com/v1；AI_MODEL 必须是你的账号实际有权调用且支持下述参数的模型 ID。兼容供应商则填它自己提供的基址和 ID，不能保证仅凭“兼容OpenAI”宣传就完全支持当前请求。

**修改 .env 后要重建容器环境，不只是 restart：**

~~~sh
# 已经部署并完成数据库迁移的实例
# 此命令会重建api/worker，扫描时应先暂停并等待在途任务完成。
docker compose up -d --no-build --force-recreate api worker
# 不要输出.env内容或docker inspect环境变量到公开日志
docker compose logs --tail=80 api worker
~~~

首次部署先配置镜像与基础数据库，再按部署说明 migrate。修改云端密钥不需要重新打包前端或构建镜像。不要 down -v，避免误删数据库及证据卷。

## 3. AI 接口准确契约

系统请求：AI_BASE_URL 去掉末尾 / 后，再拼接 /chat/completions。例如基址 https://provider.example/v1 会请求 https://provider.example/v1/chat/completions。

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

## 4. OCR 不能直接填任意厂商地址

当前原卷OCR网关请求为：POST OCR_ENDPOINT，Content-Type: application/pdf，请求体为 PDF 二进制，Bearer OCR_API_KEY（若设置）。返回：

~~~json
{"text":"识别出的试卷文字，保留题号、选项和分值","pages":3}
~~~

这不是阿里云、腾讯云、百度、PaddleOCR、Mistral 或其他厂商接口都天然支持的标准。若其要求签名、base64、文件URL、上传后异步轮询、自定义JSON、OpenAI视觉消息，则要在 Go 内增加对应 provider 适配器，把其结果映射为 text/pages。**目前没有一个已部署的通用OCR网关可以让你直接填上就用。**

不建议为此在2C4G机器再部署Python模型；优先在Go中做轻量HTTP适配，计算仍在云端。

## 5. 连续阅卡还需要独立识别适配

CARD_ENDPOINT 接收 multipart image + template，响应 token、每题 selected / state / confidence，详情见云端接口契约。

- 不信任浏览器识别的学生绑定，网关须独立读码，服务端再检查该码属于已发布考试。
- 普通文字OCR可能只识别出题号和标签，不能可靠识别填涂、擦除、多选。
- OpenAI-compatible多模态模型需要额外图像请求适配和质量评估，当前文本AI配置不会自动接管扫卡。
- 没配CARD_ENDPOINT时只能建卷 / 打印，自动扫卡会明确返回503；不能称端到端正式阅卷已经可用。
- 发送学生答题卡会涉及姓名/学号、个人码与作答信息，选供应商前确认授权、保存/训练/地域政策。

## 6. 配好以后怎么操作

1. 教师登录，确认页面AI/OCR状态。注意此处目前仅检查配置是否非空，不会主动付费探测，**“已配置”不等于连通测试成功**。
2. 创建班级 / 名册，再创建考试草稿。
3. 在“PDF与解析”上传原卷，选择文件，点击解析。原卷OCR与AI两者都配好才会受理解析任务。
4. worker调用OCR，再调用AI；页面轮询 uploaded → queued → processing → parsed / failed。
5. 查看候选，显式采纳到草稿（已有题目需要确认覆盖），检查题型、数量、分值与答案。缺值必须手工补，不允许猜。
6. 发布考试、打印个人卡。CARD网关接入并验证后才能正式连续识别入库。

只配AI而没配OCR不会完成PDF解析；没有标准答案也不会自动获得可靠答案。

## 7. 常见错误

| 现象 | 检查 |
|---|---|
| PROVIDER_NOT_CONFIGURED / 503 | OCR+AI解析缺一个，或扫卡缺CARD；确认api/worker均已force-recreate |
| 页面显示已配置但parsed失败 | 非空状态不验证鉴权，检查URL、密钥权限、协议与模型参数 |
| AI 401 / 403 | key、账号余额/权限、模型访问范围、网关鉴权方式 |
| AI 404 | 不要在AI_BASE_URL里重复/chat/completions，核对/v1和模型ID |
| AI 400 | 供应商是否支持response_format/json_object、temperature、max_tokens |
| OCR失败 | 当前是raw PDF协议，不是厂商原生JSON/签名接口；必须适配 |
| PDF一直queued | worker未运行、数据库不同、队列失败，查看api/worker日志 |
| CARD已配置但识别失败 | 协议不符、码未读出、题ID/选项/状态不完整、不属于当前考试 |

应用错误会脱敏，不把供应商密钥/完整错误响应写到教师界面。当前尚没有专门的管理员连通测试页和详细供应商调用看板，这是需要继续补的功能，不应暗示已经存在。

## 8. 下一步需要你提供的非密钥信息

- 原卷OCR供应商/模型名、API文档链接，或一份脱敏请求/响应示例。
- 大模型的Base URL、模型ID，以及是否支持Chat Completions JSON object；**不需要把真实密钥发到聊天或提交仓库**。
- 答题卡打算用专门OMR服务，还是某个云端视觉模型；对应接口文档和隐私要求。

拿到这些信息后才能做厂商原生直连适配和真实连通验收，而不只是让你配置一个尚不存在的网关。
