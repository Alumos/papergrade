# 2C4G VPS 低占用部署

## 资源原则

VPS不是此平台独占。默认云端OCR/答题卡识别/AI，本机不装模型、不装PDF渲染器、不接视频。Node仅CI构建，线上静态文件；Go API/轻量任务Worker/PostgreSQL/HTTPS网关，无Redis、MinIO、消息中间件。

当前Compose保险上限：网关64MiB、API128MiB、Worker192MiB、PostgreSQL256MiB，总640MiB。这是容器硬帽，不是预分配或已测日常占用；进程、页缓存、Docker与OS仍有额外开销，文件上传和云端并发会临时增长。不能承诺总RSS几十MB。上线需记录空闲和峰值，再决定是否上调安全帽；不要为了数字压到频繁GC/OOM。

Go软目标API96MiB、Worker128MiB；它不约束全部RSS/原生内存。PG shared_buffers32MB、work_mem2MB、max_connections12，应用池最小0、最大约3，任务低频自适应退避。Worker仅0.5核硬帽；在线进程用CPU权重而非严格切走大半机器，权重不是专属CPU保留。

暂不添加高内存监控栈或PDF工具。空闲网络请求应为零外部调用；后台任务只查小任务索引，不反复加载全部名册/历史。大数据量必须压测并分页，不把MVP单教师实现当校级多租户服务。

## 前提

Linux VPS + Docker Engine + Compose v2、绑定域名、80/443可达、足够SSD和异地备份位置。先确认其他服务端口和现有反代，已有反代可改为复用，不能让两个网关争抢80/443。公网手机相机必须HTTPS，局域网HTTP一般不行。

开发机当前没有Docker，文中命令/镜像尚需CI与真实VPS验证。绝不能把配置文件存在解释为已完成Docker运行验收。

## 首次启动

~~~sh
cp .env.example .env
# 编辑DOMAIN、账号/数据库密码及APP_ENCRYPTION_KEY；模型在登录后的后台配置。
# APP_ENCRYPTION_KEY用openssl rand -hex 32生成一次，保持稳定并单独备份。
# 数据库密码建议用openssl rand -hex 24生成，避免URL保留字符。
# 本地构建仅适合开发/测试，VPS生产推荐拉GitHub Actions镜像。
docker compose build
# 必须先迁移，api/worker启动不会自动修改数据库结构。
docker compose run --rm api migrate
docker compose up -d
docker compose ps
docker compose logs --tail=100 api worker
~~~

生产不启DEV_MODE，必须配置稳定的 APP_ENCRYPTION_KEY（至少32字节；API和Worker相同）；升级/恢复时保留，不可任意重新生成。模型接口和密钥在后台“模型与接口”设置，无需重启。

不将API8080和PG5432映射公网。数据卷保存证据、PG、Caddy证书；禁止down -v。首启应用UID10001对卷写权限必须检查；新命名卷镜像内目录权限应拷贝，现有错误权限需人工修复，不能用chmod777当常态。

## GitHub Actions与GHCR

把papergrade目录本身作为仓库根上传。ci.yml执行Go与真实PG测试/前端测试构建/Compose解析/镜像构建/一次性容器启动检查；images.yml由v*标签或手动触发，先执行真实PG与前端测试，再为linux/amd64和linux/arm64构建app与gateway镜像，发布到GHCR。此前main提交已通过真实GitHub Actions的Go/前端/Docker构建与容器启动；新增设置功能须以对应新提交的Actions结果为准。镜像发布仍需v*标签或手动触发，不等于CI检查成功就已发布。

默认使用GITHUB_TOKEN发布，需要仓库Actions具备packages:write。GHCR包的可见性按账号/组织策略设置；私有镜像VPS需用read:packages权限凭据docker login，不能把token写进Compose。

假设仓库OWNER/REPO，版本v0.1.0：镜像命名为ghcr.io/owner/repo:0.1.0，网关为同名:0.1.0-gateway。设置.env中的IMAGE为前者，Compose会自动拼接网关后缀。然后：

~~~sh
docker compose pull
docker compose run --rm api migrate
docker compose up -d --no-build
~~~

谨慎升级：先异地一致备份，暂停新扫描、等待任务结束，迁移/升级后验证登录、权限、模板与旧记录。数据库迁移可能不可逆，不能仅换旧镜像就声称回滚成功。

## 检查实际占用

~~~sh
docker stats --no-stream
docker compose ps
docker inspect $(docker compose ps -q api) --format '{{.HostConfig.Memory}}'
free -m
df -h
~~~

分别记录启动后稳定空闲10分钟、单教师连续扫描、配置云端延迟/失败、同时上传PDF、导出时的RSS、CPU、I/O和队列时延。Docker stats与宿主可用内存都要看，缓存、共享页和口径有差异。没有真机和目标VPS数据前，不承诺支持人数或每分钟吞吐。

## 文件与数据库备份

备份要覆盖PG和证据，存到不同机器/提供商，密钥独立。只备本机不能防VPS丢失。不直接复制运行中的PGDATA当有效备份。首期可低峰短维护：停止API/Worker写入，pg_dump，再归档对应证据卷，最后恢复服务；恢复后逐项核对答卷有效指针和图像。证书也按需备份。压缩/加密限速避免影响其他网站。

磁盘预算含PDF、卡图、历史重扫、导出、PG/WAL/索引、容器日志/镜像和备份暂存。按业务设置保留期/磁盘水位；不得为了释放空间悄悄删除成绩证据。单VPS依旧单点，不宣称高可用。
