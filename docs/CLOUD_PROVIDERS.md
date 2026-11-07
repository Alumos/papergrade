# 云端接口配置与数据边界

## 当前默认

**模型配置入口：登录 → 模型与接口。** URL、模型名、API Key 在后台修改，AES-GCM加密存入数据库，保存后API/Worker下一次调用即时读取，无需重建容器。部署只需一次设置稳定 APP_ENCRYPTION_KEY（随机至少32字节，单独备份）。以下旧环境变量只作第一次后台保存之前的兼容引导，不能覆盖已保存配置。完整操作见 [模型设置说明](<MODEL_SETUP.md>)。

VPS 默认只编排接口、校验响应、保存证据、确定性计分和查询。无本地OCR模型、PDF渲染/文本提取子进程、无LLM。手机负责找卡、稳定、选帧与压缩。LOCAL_OMR_ENABLED=false 为生产默认；本地OMR只作显式实验，不用它伪装云端配置完成。

识别接口未配置时页面会提示不可用，API返回真实错误；手工建卷和模板 / 名册功能仍可使用。不能填一个普通OCR地址就声称支持填涂识别，必须先验证兼容协议与真实识别质量。

## 1. 原卷 OCR 网关

环境：OCR_ENDPOINT、OCR_API_KEY。

POST 配置的固定URL，Content-Type: application/pdf，Authorization: Bearer <key>（如配置）。请求为原卷PDF字节，最多20MB。响应：

~~~json
{"text":"试卷完整OCR文本，保留题号与分值","pages":3}
~~~

目前这是通用网关协议，不是任意云厂商专有API的直接SDK。若厂商需要签名、任务轮询、对象存储URL或自定义请求结构，需部署/接入相应适配层。配置地址是服务端可信配置，不接受教师提交任意URL；禁止重定向；外部端点必须HTTPS，仅localhost/loopback测试网关可HTTP。明确供应商是否保存试卷、地域和保密条款。

## 2. AI 结构化

环境：AI_BASE_URL（包含/v1的基址）、AI_API_KEY、AI_MODEL。

OpenAI-compatible POST /chat/completions，JSON object输出。输出抽取题号、题型、选项标签、原文分值与答案；没有的答案 / 分值不得推理补造。返回questions/warnings，经JSON与业务校验，仅形成草稿，教师确认才能发布。文本上限120KB、响应上限2MB、默认90秒超时；不自动无限重试、不把供应商返回错误原文暴露给客户端。

## 3. 学生答题卡云端识别网关

环境：CARD_ENDPOINT、CARD_API_KEY。与原卷OCR端点分离。

POST multipart/form-data：
- image：JPEG/PNG证据，≤4MB、≤8MP；服务端只读头部，不默认完整解码。
- template：JSON，包含schema_version=card-v1、manifest几何（不含标准答案）、questions（仅id/type/options）。

网关必须根据图像独立校验定位点、模板布局、二维码和填涂，不能回显手机candidate token或让LLM随意猜测。返回：

~~~json
{
  "token":"从图像读取的服务端已发行个人卡opaque token",
  "answers":[
    {"question_id":"q1","selected":["A"],"state":"answered","confidence":0.99},
    {"question_id":"q2","selected":[],"state":"uncertain","confidence":0.4}
  ],
  "warnings":[]
}
~~~

要求每题恰好一条、ID完整、选项合法、状态明确。state是answered/blank/invalid_multiple/uncertain。模糊必须uncertain，不得猜成blank。confidence仅供应商提供的线索，不是已验证正确概率。Go验证所有ID/选项/状态，并用token在已发布考试名单内绑定学生；匹配不成不能猜。没有把标准答案送给云端，云端不返回可直接入库的分数，Go按冻结规则计分。

该路径会发送学生答题卡给所选供应商，可能包含纸面姓名/学号。上线前必须完成学校/教师授权、数据驻留、保留/训练策略审查；若要脱敏需裁切而仍保留可识别token的专用设计，不能虚称传输没有个人信息。系统不会默认同时将图像发送多家。

## 4. 测试与费用

仓库中的httptest仅验证协议、上限和错误处理，不证明任何真实供应商识别质量。未指定并接入真实供应商之前，不宣称“云端自动阅卷已验收”。本地合成OMR测试也不等于iPhone/Android真机测试。

原卷OCR按页/图，AI按真实token/图像档位，云端卡识别按供应商协议计费；现在逐卡走云端意味着确有每卡接口成本，相比原开发文档的本地OMR路线，用费用换低VPS内存/CPU。不要再使用“所有扫卡零云接口费用”的成本假设。外部接口超时可能重复计费，有限重试和幂等任务只降低风险，无法保证供应商exactly-once。
