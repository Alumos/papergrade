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

## 一次有限的本机资源观测

macOS 27.2 arm64，编译后的Go API、显式memory_dev、云端未配。执行启动 + HTTP名册/发布/SVG冒烟 + 100次readiness请求后正常退出，通过os.wait4统计：两次有限运行单进程峰值RSS约15.48–15.62 MiB，最近一次user CPU 0.147秒、sys CPU 0.013秒，约0.394秒。脚本scripts/measure-dev.py可复现（自动生成一次性随机测试凭据，不使用真实账号）。

**这不是生产空闲占用、不是Docker总内存、不是2C4G容量证明。** 不含PostgreSQL、Worker、Caddy、OS/缓存、真云端和手机；也不是长时间稳态测试。只说明轻量Go API本身没有常驻OCR模型。生产仍须在真实VPS分别测空闲10分钟和实际工作峰值。

## 明确未完成的环境验收

- YAML 已经通过现有 Prettier YAML 解析器语法校验，但语法解析不替代 Compose 语义 / 运行校验。
- 本机仍无Docker Engine，Docker构建/Compose运行未本地验收；PostgreSQL原先SKIP已由后续真实隔离实例测试补足。
- CI与镜像发布workflow已配置PG服务+TEST_DATABASE_URL，远程尚未推送/运行。Compose语义解析、镜像真正构建和容器启动尚需CI/目标机执行。
- 未提供真实OCR/AI/card供应商密钥和具体协议映射；httptest不证明云端识别质量或计费。
- 未执行实体打印+iPhone/Android Safari/Chrome相机、光照/遮挡/发热/长扫与弱网验收。
- 单教师MVP不等于原文档全部P0已实现；延期项以README和实际capabilities为准，不返回假成功。
