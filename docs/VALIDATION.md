# 验证记录（本地首版，持续更新）

## 已执行

- Go全模块：go vet ./...、go test -race -count=1 ./... exit0（2026-11-07）。
- 后端HTTP tests通过：登录/Origin、名册原子、发布冻结、真实httptest网络云端协议、幂等、学生唯一有效、冲突不覆盖、复核、PDF原卷及候选解析、CSV、文件权限。
- provider测试：真实HTTP multipart、未知/缺失题目及非法选项拒绝、结构化AI、响应错误脱敏、超时、重定向禁止、公网HTTP拒绝。
- 视觉包完整合成 / race / vet通过：marker/透视/QR、单选多选空答/不确定/白遮挡、JPEG、旋转、低质量拒识。它是默认关闭的本地实验，不代表云端或真机达标。
- Linux amd64 与 arm64 均已 CGO_ENABLED=0 静态交叉编译成功。
- 独立编译API实际HTTP smoke：登录、班级/前导零名册、建卷、发布、manifest、个人SVG卡、空结果、注销通过。
- 官方shadcn CLI init/add真实执行成功（4.23.0，radix-nova/neutral），14个官方基础组件。原自制UI已删除，Tailwind4与lock同步；13项前端单测、类型检查、生产构建、frozen lock安装通过。
- 真实浏览器完成登录、建班、001/002名单导入、建卷、发布、个人SVG、空成绩、未配云端503、两题答案A→B显式重评；移动与桌面界面截图已检查。
- 已下载并完整性校验隔离PostgreSQL17.6测试包（仅.cache，不加入生产镜像），真实rollback、unique、第二连接池读取、租约fencing、3次crash死信均PASS；含TEST_DATABASE_URL全项目race+vet通过。
- 真实PostgreSQL + 独立API / Worker进程E2E通过：202异步入队、受控云端协议fixture、正式提交、幂等、答案重评、API重启后成绩保留。fixture不证明真实云服务OMR准确率。

## 2026-11-08 后台模型设置验证

- Go全量 race + vet 通过；真实PostgreSQL17.6随机schema验证两独立API/Worker热更新、重启后解密、错主密钥503、AES-GCM篡改检测、空密钥保留/显式清除、revision并发仅一方获胜、未认证401与跨Origin403。
- 独立API/Worker进程E2E通过：不配置任何模型环境变量，后台保存卡网关再修改URL，已运行Worker使用新URL及保留密钥，API重启后设置和成绩仍可读。测试用受控fixture，不代表真实供应商识别质量。
- 官方设置页TypeScript、20项前端测试、生产构建通过。真实浏览器完成登录→设置→输入AI基址/模型/测试密钥→确认保存；revision 0→1、来源变为后台配置、顶部AI徽章立即刷新、密码框清空且已存密钥不在页面DOM。
- 地址/鉴权测试仅AI GET /models与网关HEAD，不发试卷，不进行推理，不能作为识别质量通过的证据。

## 2026-11-08 v0.2原生Ollama/双联学号卡

- 最终28项前端测试、TypeScript、生产构建、显式1440分钟发布时间策略frozen安装通过；Go全量race/vet与真实PostgreSQL测试通过。
- 真实用户DDNS HTTPS（有效证书，不使用-k）返回Ollama0.40.15；同一浏览器试卷上传双页PDF→PDF.js Worker准备2页→真实NAS Xiaomi OCR逐页识别完成2/2、无页错误（10:11开始，10:18:35观察到完成，包含轮询等待，非精确推理时长）→OpenAI格式HTTP fixture分析→教师采纳并补答案B→发布。未提供付费AI密钥，AI是明确标记的fixture，不能称真实供应商已验收。
- 同一OCR试卷的真实SVG经浏览器渲染、填学号001/B，实际Web Worker自动找角识别001/B，再发同张JPEG给Go独立复核：HTTP200/committed/score_units200/max200/review0，请求约74ms（本机合成图，不是手机拍纸速度或准确率）。
- A4双联实际PDF已下载并用PDF.js重渲染：1页约210×297mm、中文/学号/裁切线正常。最大40题/12位学号SVG已读图审查。PDF为300dpi固定模板栅格嵌入，不冒称矢量。
- CI新脚本v02-e2e使用独立随机schema、真实API/Worker/PG、Ollama+AIHTTP协议fixture、教师确认/缺答案阻断、A4双联、0001/0002真实像素OMR、幂等、重评分与API重启成绩保留。原v0.1 postgres-e2e脚本只历史参考。
- NAS原LAN测试中途因客户端切换网段不可达已取消；没有把该失败当OCR成功。正式DDNS链路由真实双页PDF完成验证，不再预置旧LAN IP。所有NAS地址/账号/样图只在后台或本机忽略目录，不进代码默认。
- v2二维码是全班相同考试ID，不校验学生；学号填错为另一合法号的剩余风险通过教师核对处理。真实手机/实体2B纸卡/长期热稳定仍未验收。


## 2026-11-08 英语专项与真实模型验证

- 新增 a5-v3 英语半A4卡：固定4位学号、2–7选项、60题、题组标题、A4双联；容量超限明确拒绝发布。Go 测试覆盖：字母印在方框内时空白框仍判 blank（采样外环）、填涂正确识别、宽高比拒绝未裁双联页、JSON 往返校验。
- 真实 OpenAI-compatible 模型（用户提供 `deepseek-v4.1-flash`，端到端 `HTTPS /v1/chat/completions`）已实际调用：成功抽取“听力（ABC）/阅读理解（ABCD）/七选五（A–G）/知识运用（ABCD）”四组结构并带题组名。该验证只说明协议与抽取可用，不代表所有供应商或所有试卷都准确。
- 端到端脚本新增英语链路：发布 a5-v3 60题 → 生成双联SVG → 用实际像素填涂卡提交 → Go 独立识别学号 1932 与 60 题并计分满分。仍使用 HTTP fixture 作为 OCR/AI 协议替身（真实 NAS OCR 与真实模型另行单独验证）。
- 仍未完成：真实手机拍摄、实体2B打印、不同纸张/光线条件下的大样本准确率。

## 一次有限的本机资源观测

macOS 27.2 arm64，编译后的Go API、显式memory_dev、云端未配。执行启动 + HTTP名册/发布/SVG冒烟 + 100次readiness请求后正常退出，通过os.wait4统计：两次有限运行单进程峰值RSS约15.48–15.62 MiB，最近一次user CPU 0.147秒、sys CPU 0.013秒，约0.394秒。脚本scripts/measure-dev.py可复现（自动生成一次性随机测试凭据，不使用真实账号）。

**这不是生产空闲占用、不是Docker总内存、不是2C4G容量证明。** 不含PostgreSQL、Worker、Caddy、OS/缓存、真云端和手机；也不是长时间稳态测试。只说明轻量Go API本身没有常驻OCR模型。生产仍须在真实VPS分别测空闲10分钟和实际工作峰值。

## 明确未完成的环境验收

- YAML 已经通过现有 Prettier YAML 解析器语法校验，但语法解析不替代 Compose 语义 / 运行校验。
- 本机仍无Docker Engine，Docker构建/Compose运行未本地验收；PostgreSQL原先SKIP已由后续真实隔离实例测试补足。
- 之前main提交已通过GitHub Actions的Go/PostgreSQL、前端、Docker构建与一次性容器检查；后台设置新提交仍需对应CI验证。镜像发布与目标VPS性能/真机验收是另外的步骤。
- 未提供真实OCR/AI/card供应商密钥和具体协议映射；httptest不证明云端识别质量或计费。
- 未执行实体打印+iPhone/Android Safari/Chrome相机、光照/遮挡/发热/长扫与弱网验收。
- 单教师MVP不等于原文档全部P0已实现；延期项以README和实际capabilities为准，不返回假成功。
