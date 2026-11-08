# Papergrade · 智能阅卷

面向教师手机的智能阅卷平台 v0.2 试点版。Go 后端、React / Vite 静态前端、官方 shadcn/ui（CLI 4.23.0，radix-nova / neutral）、单文件 SQLite、Docker Compose。

**原卷OCR在NAS／云端，标准卡用OMR。** 手机负责PDF分页、视频门控和Worker填涂预识别；Go独立复核选中单卡的学号与答案、计分入库。不上传视频，不自建OCR／LLM，不在VPS渲染PDF。

## 项目状态

目前已落地的首版业务：
- 单教师登录、班级/名单 CSV 导入、题目草稿与发布冻结。
- 后台“模型与接口”管理OCR、OpenAI-compatible AI、答题卡网关；密钥加密/脱敏，保留或清除，版本冲突保护，保存后API/Worker热更新。
- 固定学号填涂答题卡：`a5-v2`（最多40题、1–12位数字学号、前导零保留）与英语专用 `a5-v3`（60题、2–7选项、A–G七选五、学号固定4位），A4上下双联SVG／PDF，裁开单卡扫描；二维码仅考试ID。
- 单选、多选、判断的确定性整数评分、复核历史、防重复和冲突不覆盖。
- PDF/图片原件入库，浏览器PDF.js逐页准备；后台Ollama原生OCR可选密钥/可改DDNS/默认空地址，页进度与显式恢复，Markdown／LaTeX与OpenAI候选确认。
- 标准答案纠错后版本化重评分，保留原发布模板、评分历史和待复核状态。
- 持久任务队列、单文件 SQLite 存储（无数据库服务）、私有文件、统计与CSV。
- 手机连续相机门控、稳定选帧、IndexedDB恢复与事务后正式反馈（官方组件重建及基础业务浏览器联调已完成）。

OCR未配置时解析真实返回不可用；新学号卡无需CARD或大模型，用浏览器OMR+Go复核。旧a4-v1保持兼容。付费AI、真实手机和实体2B卡准确率未验收前不称生产全面完成。

## 本地开发

需要 Go 1.24+、Node.js 24 LTS、pnpm 11.7.0。生产只需设置 SQLITE_PATH 指向一个数据库文件，首次启动自动建表；没有 SQLITE_PATH 时只允许显式 DEV_MODE=true 的临时内存模式，重启丢失，不用于真实成绩。

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

生产不启DEV_MODE，必须设置 SQLITE_PATH、PUBLIC_ORIGIN、账号、强密码及稳定 APP_ENCRYPTION_KEY（openssl rand -hex 32 生成一次，单独备份）。OCR/AI/答题卡模型配置登录后在“模型与接口”修改，日常保存无需重启。API/Worker 共享同一个 SQLite 文件与证据目录，首次启动自动建表；生产有独立 Worker，内存开发模式不能跨进程共享数据。

## 文档

- **[v0.2 发布与操作说明](<docs/RELEASE_V02.md>)**：Ollama原生、PDF浏览器分页、学号双联与手机OMR。

- [最新低资源约束](<docs/LOW_MEMORY_REVISION.md>)：覆盖原设计的默认路径。
- [部署与GHCR](<docs/DEPLOYMENT.md>)：2C4G资源保险帽、域名、卷、备份与升级。
- **[OCR / OpenAI 模型接入步骤](<docs/MODEL_SETUP.md>)**：后台配置位置、保存热更新、加密主密钥、URL格式、协议限制和所缺厂商适配。
- [云端接口契约](<docs/CLOUD_PROVIDERS.md>)：OCR、AI、答题卡识别配置及隐私/费用边界。
- [实施API契约](<docs/IMPLEMENTATION_CONTRACT.md>)。
- [验证记录与尚未验收范围](<docs/VALIDATION.md>)。
- [后端说明与明确延期项](<internal/app/README.md>)。
- [标准卡 / 实验视觉](<internal/vision/README.md>)：只合成测试，不宣称真机准确率。

## Docker / GitHub Actions

三个常驻服务：HTTPS静态网关、Go API、轻量Worker，加一个 SQLite 数据卷。无数据库容器、无Node生产运行时、无Redis/MinIO。容器内存上限合计约450MiB是故障保护，不是预分配、真实RSS或资源达标证明。生产镜像不包含本地OCR模型/PDF工具。

代码仓库：[Alumos/papergrade](https://github.com/Alumos/papergrade)。推送 main 自动执行 Go / SQLite 与前端测试、Docker 构建及一次性容器检查；发布 v* 标签或手动运行 Publish Docker images 时构建并推送 GHCR 的 linux/amd64、linux/arm64 镜像。当前构建状态见 [GitHub Actions](https://github.com/Alumos/papergrade/actions)，不能把代码已上传当作镜像已发布。开发主机无 Docker，容器运行以 CI / 目标 VPS 的真实结果为准。

## 明确首版边界

不是原开发文档中所有P0的完整交付：v0.2单教师、40题内半A4学号卡；暂不包含多租户、XLSX、任意旧卡适配、无人确认自动采纳AI候选、主观题评分、冲突卡显式替换、完整预算/保留策略管理。后续应按优先级补齐，不能把未做接口标成成功。

证据目录已有耐久写入但孤儿清理/长期保留管理需完善；上线前安排磁盘配额、异地一致备份、真实供应商及学生数据外发授权、支持手机矩阵测试和目标VPS占用压测。
