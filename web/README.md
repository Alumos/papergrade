# 阅见 Web

React 19 + TypeScript + Vite 真实 API 静态前端，已按用户要求废弃旧自制 UI 和业务布局，从官方 shadcn/ui CLI 4.23.0 重新初始化并生成14个官方组件。采用官方 radix-nova / neutral 黑白主题、Tailwind CSS 4 + 官方 Vite 插件、Geist 本地字体。基础组件保留 CLI 生成源码，业务页面仅组合这些官方组件；无演示数据、无虚构上传或识别成功。

官方依据：https://ui.shadcn.com/docs/installation/vite 与 https://ui.shadcn.com/docs/cli。
实际执行成功命令：

```sh
pnpm dlx shadcn@latest init --base radix --template vite --preset nova --yes --force --no-monorepo --no-rtl
pnpm dlx shadcn@latest add button card badge input label textarea tabs dialog alert select table separator skeleton checkbox --yes --overwrite
```

当前 CLI 的官方预设名为 nova（components.json style=radix-nova），不是旧版 new-york，故不冒称生成了旧版；baseColor 为 neutral。配置由 CLI 写入 components.json，路径 src/components/ui/*。旧 src/pages、src/components/ui.tsx 与旧绿色样式已移除。安全 API / CSV / 队列逻辑复用为独立模块，扫描控制抽出 features/scan/useScanController.ts。扫描 UI 和 jsQR 经 React.lazy 按需加载，不进入首页扫描大包。

## 开发与构建

使用 packageManager 固定的 pnpm 11.7.0：`pnpm install`、`pnpm typecheck`、`pnpm test`、`pnpm build`、`pnpm dev`。开发 Vite 仅监听 localhost，/api 保留 Origin 代理到 127.0.0.1:8080（localhost 的 IPv4 回环，避免 IPv6 监听差异）。生产发布 dist 静态产物，同源反代 /api，不运行常驻 Node。远程手机相机必须 HTTPS；本机 localhost 例外。

## 教师流程

Cookie 登录、真实能力/内存模式提示；创建班级、CSV/TSV 文本名册导入（保留前导零、重复拒绝）；草稿配置单选/多选/判断（100整数单位=1分）并冻结发布；服务端个人 SVG 卡 A4 打印，浏览器可另存 PDF；结果统计/CSV；按 revision 提交复核选项及原因，不提交分数；PDF 真实上传及解析状态，未配置供应商返回真实错误。服务能力启用后可在官方 Dialog 显式采纳候选到草稿（已有题须勾选 replace 确认，缺答案/分值仍待核对）；客观题重评预览原/新答案、理由及 revision，保留旧 release 与历史，不伪称主观题已批。

## 连续相机

getUserMedia 一次启动，视频只在 DOM；帧不存 React state。每650ms小尺寸 canvas 检查亮度、清晰度、黑色连通块四角、jsQR 个人token，连续三帧同token且稳定后才选帧JPEG（最长边1800，质量0.86）。这是轻量启发式取景门控，不是正式OMR。vision manifest为normalized a4-v1，QR raw opaque token。服务端/云端必须独立验证，不信任candidate token。

IndexedDB先存Blob和client_capture_id，最多3个未终结任务，单上传/轮询；Idempotency-Key与client_capture_id相同，网络错误保留原请求手动重试。已选token防止重复自动入队。仅committed绿灯；新committed且非deduplicated才发声，duplicate不计数。后台释放相机，前台用户继续；已保存任务恢复查询。暂停不撤销服务端已受理请求。JPEG图片上传可兜底，仍由服务端审核。

## 真实限制

固定A4卡试验；未实测真实手机、反光、折痕、复杂透视、Safari权限与音频、长期离线。低分辨率黑块会受背景文字干扰，jsQR失败不会盲拍，改用上传。不宣称任意卡准确率。云端识别需专用结构化供应商endpoint；未配不会假成功。OCR/AI解析不支持时可手工建卷。IndexedDB需浏览器支持，清站点数据会丢待处理图像；应控制共享设备访问。复核通过认证接口读取原始答卷；重评与采纳按钮以真实 capabilities 为准；未配置云端无法进行真实供应商识别验收，不虚构成功。
