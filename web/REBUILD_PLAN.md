# 官方 shadcn/ui 重建记录（已完成）

> 本文保留当时的设计计划；官方 CLI 生成、旧 UI 删除、新 UI 构建与真实浏览器联调均已完成，最新状态以 [Web 说明](<README.md>) 为准。下文的“等待”是历史计划，不是当前阻塞。

## 边界

只写 papergrade/web。等待 Lead 获准执行官方 CLI，当前不删除旧源文件，防止暂时没有可构建前端。官方安装文档已读取：https://ui.shadcn.com/docs/installation/vite 及 https://ui.shadcn.com/docs/tailwind-v4。现有 Tailwind 3 项目仍受 CLI 支持，可由官方按项目生成兼容组件。

## 保留的逻辑（非布局）

- src/api.ts：真实 Cookie API，JSON错误、503不伪成功、同源凭据、multipart幂等头、超时、会话过期事件、串行卡片取回。
- src/types.ts / hooks.ts / lib.ts：合同类型、可取消查询、CSV解析与前导零/重复校验、100单位=1分、终态判断。
- src/scan/quality.ts：jsQR不透明token、亮度/清晰度、黑色候选四角、连续三帧稳定、同token已选防重。仅门控不是正式识别。
- src/scan/store.ts：IndexedDB先落Blob/client_capture_id，按exam索引，恢复、防重、终态清理Blob。
- Scanner内与布局分离的控制逻辑：一次启动相机、低频小canvas、最长边1800 JPEG、最多3项、单上传/轮询、原幂等键重试、后台释放、失败显式重拍、仅committed绿灯及新committed音。
- 已有13单测：CSV/units/terminal、API边界、稳定换卡；重建继续运行。

## 新结构

- src/components/ui/*：只能来自官方 shadcn CLI，至少 button/card/badge/input/label/textarea/tabs/dialog/alert/select/table/separator/skeleton（建议checkbox）。不保留原components/ui.tsx自制基础库。
- src/components/app/*：官方组件的业务组合（状态提示、空状态、加载、分数、API错误）；不得手工重造基础组件。
- src/features/auth/Login.tsx：官方Card/Header/Title/Description/Content/Footer、Input、Label、Button、Alert；白底黑字单列可达，桌面无旧绿色营销区。
- src/features/classes/ClassesPage.tsx：标题、官方创建Dialog、班级Select、CSV Textarea、真实Table；手机单列，名单表可横向滚动。
- src/features/exams/ExamsPage.tsx：官网黑白应用header/nav、官方Tabs all/published/draft、Card列表、Skeleton。
- src/features/exams/ExamWorkspace.tsx：官方Tabs配置/个人卡/扫描/结果/PDF，发布确认Dialog。
- src/features/exams/ExamForm.tsx：Input/Label/Select/Checkbox，单多判断题、units、部分分；40题/5项限制。
- src/features/print/PrintPanel.tsx：真正SVG个人卡认证打印，官方Checkbox/Table，A4 100%，无假PDF endpoint。
- src/features/results/ResultsPage.tsx：官方统计Cards/Table/Tabs，原图复核Dialog、理由Textarea、revision提交，无客户端计分。
- src/features/documents/DocumentsPage.tsx：PDF Input、Select明确document_id、真实状态Alert、只读候选，人工确认后录入。
- src/features/scan/ScanPage.tsx + useScanController.ts：原安全队列逻辑抽hook；DOM video帧不入state，官方Cards/Alerts/Buttons/Tables；仅committed语义绿，其他应用中性黑白。

## 设计与验收

官网new-york/neutral CSS tokens：background白、foreground近黑、muted中性灰、border细灰，默认官方radius；布局仅辅助，不覆盖官方Input/Button视觉。桌面最大宽度、细边header，手机折行tabs及安全边距；所有表单有Label，焦点和键盘行为由Radix保证。删除前只验证明确绝对路径 /Users/alumos/Documents/Projects/智能阅卷/papergrade/web/src/components/ui.tsx 等属于web，不碰backend/deploy。重建后pnpm typecheck/test/build及真实浏览器桌面/移动截图；后端未可用不能声称联调通过。
