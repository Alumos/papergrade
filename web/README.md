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

getUserMedia 一次启动，视频只在 DOM；帧不存 React state。650ms 低频小尺寸 canvas 做质量/稳定门控，之后高分辨图像交真正的 OMR WebWorker：四角定位、四方向黑白参考、透视 homography、灰度覆盖采样、学号逐列十进制与客观题解码。a5-v2 半A4卡二维码全班相同，仅匹配试卷；学生唯一身份来自学号填涂，保留前导零。v2以学号+画面位移/冷却防换卡误拍；旧a4-v1仍按原二维码流程。浏览器结果只是预览，不上传分数；Go独立从JPEG复核，只有committed绿灯/音。全张双联页应拒绝，必须裁成一份210×148.5mm单卡。

IndexedDB先存Blob和client_capture_id，最多3个未终结任务，单上传/轮询；Idempotency-Key与client_capture_id相同，网络错误保留原请求手动重试。v2已选学号防止重复自动入队，不把共用试卷QR当学生锁。仅committed绿灯；新committed且非deduplicated才发声，duplicate不计数。后台释放相机，前台用户继续；已保存任务恢复查询。暂停不撤销服务端已受理请求。JPEG图片上传可兜底，仍由服务端审核。

## 真实限制

固定A4双联a5-v2卡试验；未实测真实手机、反光、折痕、复杂透视、Safari权限与音频、长期离线。低分辨率取景黑块会受背景文字干扰；v2身份不确定不会自动绑定，图片上传可进入人工复核。不宣称任意卡准确率。v2 Go标准卡OMR无需Card网关；旧卡云识别仍需专用供应商，未配不假成功。OCR/AI解析不支持时可手工建卷。IndexedDB需浏览器支持，清站点数据会丢待处理图像；应控制共享设备访问。复核通过认证接口读取原始答卷；重评与采纳按钮以真实 capabilities 为准；未配置云端无法进行真实供应商识别验收，不虚构成功。

## v0.2 文档与打印

OCR设置支持Gateway/原生Ollama的type/model/base URL，原生密钥可空，不预置NAS/DDNS地址。公网Ollama建议HTTPS+鉴权；VPS无法直接访问家中私网，需VPN或反向代理。PDF.js及worker按需加载，PDF≤30页，顺序转最长边2048 JPEG≤4MB上传；GET pages恢复已准备页，重新选择原件会字节比对防混页，解析失败需显式重试，不盲目重复推理。逐页OCR文本及最终候选用react-markdown（skipHtml）+KaTeX（trust:false）显示，Ollama LaTeX分隔符规范化；外部图片不自动请求。

打印根据真实print-sheet每两人一页A4，末页奇数下半空白，不缩小旧卡。可下载原SVG、浏览器打印/另存PDF，以及真正jsPDF生成的A4 PDF（将认证模板SVG在本机300dpi离屏渲染嵌入；不是截应用UI，避免默认PDF字体丢中文）。实际浏览器PDF.js回读验证210×297mm、中文与填涂区可见，下载PDF是栅格模板非矢量文字。

## 验证边界

本轮真实浏览器完成DDNS Ollama两页OCR（可选密钥为空）→本地HTTP OpenAI协议fixture候选→教师采纳并补标准答案→发布→同一release单卡SVG合成填号001/B→真实WebWorker与Go均识别→200 committed 2/2分。AI fixture不是付费真实模型；合成填涂不是手机拍摄，真实2B纸卡与手机质量仍需实物验收。测试包含前导零、选项、透视skew、空/多涂/缺位、共享QR不同学生与冷却门控。pnpm11.7.0 minimumReleaseAge1440维持，parse5稳定8.0.0 override，core-js install script显式拒绝。
