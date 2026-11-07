# Papergrade · 智能阅卷

面向教师手机的智能阅卷平台首版开发中。Go 后端、React / Vite 静态前端、官方 shadcn/ui（CLI 4.23.0，radix-nova / neutral）、PostgreSQL、Docker Compose。

**默认云端识别，低 VPS 日常开销。** 手机负责持续视频门控和自动选帧；VPS 接收单张证据，调用云端、验证结构/身份、规则计分和保存结果。不上传视频，不自建 OCR / LLM，不默认本地渲染 PDF 或运行 OMR。

## 项目状态

目前已落地的首版业务：
- 单教师登录、班级/名单 CSV 导入、题目草稿与发布冻结。
- A4 单页个人码标准卡（最多40题），SVG浏览器打印。
- 单选、多选、判断的确定性整数评分、复核历史、防重复和冲突不覆盖。
- PDF 原卷上传、可配置 OCR + AI 解析，候选经显式确认可采纳为草稿，缺值不猜。
- 标准答案纠错后版本化重评分，保留原发布模板、评分历史和待复核状态。
- 持久任务队列、PostgreSQL独立实体存储、私有文件、统计与CSV。
- 手机连续相机门控、稳定选帧、IndexedDB恢复与事务后正式反馈（官方组件重建及基础业务浏览器联调已完成）。

未配置供应商时真实返回不可用，不生成假识别/假成绩。普通OCR未必能识别填涂卡，需实现并验证专用card-v1网关。真实供应商、iOS/Android相机和目标VPS未验收前不称生产完成。

## 本地开发

需要 Go 1.24+、Node.js 24 LTS、pnpm 11.7.0。生产需要PostgreSQL 17；没有DATABASE_URL时只允许显式DEV_MODE=true的临时内存模式，重启丢失，不用于真实成绩。

~~~sh
# 在papergrade根目录；先设置临时开发凭据（12–72字节）
export DEV_MODE=true
export ADMIN_USERNAME=teacher
export ADMIN_PASSWORD="$(openssl rand -hex 24)"
export SERVER_ADDR=127.0.0.1:8080
export PUBLIC_ORIGIN=http://localhost:5173
go run ./cmd/papergrade api

# 另一个终端
cd web
pnpm install --frozen-lockfile
pnpm run dev
~~~

Vite仅供localhost开发，/api代理到8080。访问http://localhost:5173。手机远程相机需HTTPS，不能把本机页面能打开当成手机取流已验证。

生产不启DEV_MODE，必须设置DATABASE_URL、PUBLIC_ORIGIN、账号和强密码。API/Worker共享私有证据目录，首次先migrate再启动；生产有独立Worker，内存开发模式不能跨进程共享数据。

## 文档

- [最新低资源约束](<docs/LOW_MEMORY_REVISION.md>)：覆盖原设计的默认路径。
- [部署与GHCR](<docs/DEPLOYMENT.md>)：2C4G资源保险帽、域名、卷、备份与升级。
- **[OCR / OpenAI 模型接入步骤](<docs/MODEL_SETUP.md>)**：配置位置、URL格式、重建容器、协议限制和所缺厂商适配。
- [云端接口契约](<docs/CLOUD_PROVIDERS.md>)：OCR、AI、答题卡识别配置及隐私/费用边界。
- [实施API契约](<docs/IMPLEMENTATION_CONTRACT.md>)。
- [验证记录与尚未验收范围](<docs/VALIDATION.md>)。
- [后端说明与明确延期项](<internal/app/README.md>)。
- [标准卡 / 实验视觉](<internal/vision/README.md>)：只合成测试，不宣称真机准确率。

## Docker / GitHub Actions

四个常驻服务：HTTPS静态网关、Go API、轻量Worker、PostgreSQL。无Node生产运行时、无Redis/MinIO。容器内存上限合计640MiB是故障保护，不是预分配、真实RSS或资源达标证明。生产镜像不包含本地OCR模型/PDF工具。

代码仓库：[Alumos/papergrade](https://github.com/Alumos/papergrade)。推送 main 自动执行 Go / PostgreSQL 与前端测试、Docker 构建及一次性容器检查；发布 v* 标签或手动运行 Publish Docker images 时构建并推送 GHCR 的 linux/amd64、linux/arm64 镜像。当前构建状态见 [GitHub Actions](https://github.com/Alumos/papergrade/actions)，不能把代码已上传当作镜像已发布。开发主机无 Docker，容器运行以 CI / 目标 VPS 的真实结果为准。

## 明确首版边界

不是原开发文档中所有P0的完整交付：首版单教师、个人码标准卡、40题内单页；暂不包含多租户、XLSX、通用学号填涂卡、批量个人PDF制版、无人确认自动采纳AI候选、主观题评分、冲突卡显式替换、完整预算/保留策略管理。后续应按优先级补齐，不能把未做接口标成成功。

证据目录已有耐久写入但孤儿清理/长期保留管理需完善；上线前安排磁盘配额、异地一致备份、真实供应商及学生数据外发授权、支持手机矩阵测试和目标VPS占用压测。
