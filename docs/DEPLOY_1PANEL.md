# 1Panel 部署步骤（SQLite 单文件）

编排文件：[deploy/compose.1panel.yaml](<../deploy/compose.1panel.yaml>)。没有数据库容器，整个系统只有 API、Worker、网关三个容器加一个数据卷。

## 1. 先替换 4 处

```sh
openssl rand -hex 32    # 生成加密主密钥
```

| 位置 | 改成 |
|---|---|
| `APP_ENCRYPTION_KEY` | `openssl rand -hex 32` 的输出 |
| `ADMIN_PASSWORD` | 你的登录密码，至少 12 位 |
| `PUBLIC_ORIGIN` | 实际访问的 `https://域名`，结尾不带斜杠 |
| `ADMIN_USERNAME` | 登录名，默认 `teacher` |

`APP_ENCRYPTION_KEY` 一旦启用就**不能再改**：它解密后台保存的模型密钥，换了以后已存密钥全部读不出来。请单独抄一份备份。

## 2. 粘贴部署

1Panel → 容器 → 编排 → 创建编排，名称 `papergrade`，粘贴 YAML，确认部署。

首次启动会自动建表（SQLite 的 `CREATE TABLE IF NOT EXISTS`），不需要单独的 migrate 步骤。数据库文件在数据卷里：`/app/data/papergrade.db`。

## 3. 配置反向代理

1Panel → 网站 → 创建网站（绑定域名并开启 HTTPS）→ 反向代理：

- 代理地址：`http://127.0.0.1:18080`
- 发送域名：`$host`

gateway 容器内部是纯 HTTP（`DOMAIN=":8090"` 只给端口，Caddy 不会申请证书），TLS 全部由 1Panel 负责。

若 1Panel 的 OpenResty 运行在容器内、访问不到 `127.0.0.1:18080`，把 gateway 的端口映射改为 `"18080:8090"`，代理地址改用宿主机内网 IP。

必须保留原始 `Host` 与 `Origin`。应用会校验 Origin 是否等于 `PUBLIC_ORIGIN`，不一致会返回 403 `CSRF_ORIGIN`。

## 4. 首次登录与模型配置

用 `ADMIN_USERNAME` / `ADMIN_PASSWORD` 登录 → 「模型与服务设置」：

- **OCR**：类型选 Ollama；地址填 Ollama 基址（不带 `/api/chat`）；模型填 NAS 上实际的模型名；原生 Ollama 一般无密钥，留空。
- **AI**：填 OpenAI 兼容基址（通常以 `/v1` 结尾）、模型名、密钥。

保存后 API 与 Worker 立即生效，无需重启。模型地址与密钥加密存在 SQLite 里，不写进 compose。

## 5. 验收

```sh
docker logs papergrade-api --tail 30
docker logs papergrade-worker --tail 30
curl -s http://127.0.0.1:18080/readyz
```

浏览器打开域名 → 登录 → 建班级（学号位数统一，英语卷要求 4 位数字）→ 传试卷 → 解析 → 采纳 → 发布 → 打印答题卡。

## 6. 备份与升级

备份只要三样：`evidence` 卷（里面有 `papergrade.db` 和证据图片）、`APP_ENCRYPTION_KEY`。

```sh
# 一致性备份数据库（容器内执行，避免直接拷贝正在写的文件）
docker exec papergrade-api sh -c "ls /app/data"
```

最稳妥的做法是停掉 worker 与 api 再复制卷；数据量小，几秒钟即可。

升级：把两处镜像标签换成新版本号重新部署即可，**不要**删除 `evidence` 卷。


## 常见问题：用 `http://服务器IP:端口` 打不开或登录失败

这不是部署失败，是设计上的限制，两个原因叠加：

1. **端口只绑在 127.0.0.1**：编排里写的是 `"127.0.0.1:10086:8090"`，只允许本机访问。要对外监听得写成 `"10086:8090"`。
2. **即使放开端口，登录也会失败**：
   - 生产模式要求 `PUBLIC_ORIGIN` 是 `https://`，浏览器发来的 Origin 必须与它完全一致，否则返回 403 `CSRF_ORIGIN`；
   - 生产模式的会话 Cookie 带 `Secure`，浏览器在纯 HTTP 下不会保存，登录后立刻又变成未登录。

所以 `http://IP:端口` 这种方式**无法登录**，不是配置错了。

### 正确做法：用域名 + HTTPS

1Panel → 网站 → 创建网站（绑定域名、申请证书开启 HTTPS）→ 反向代理到 `http://127.0.0.1:10086`，发送域名 `$host`。然后把 `PUBLIC_ORIGIN` 改成 `https://你的域名`。

### 临时自测：SSH 隧道

如果只是想先在本地看一眼界面，不要暴露公网：

```sh
# 本机执行，把服务器的 10086 映射到本地
ssh -L 10086:127.0.0.1:10086 root@你的服务器
```

然后浏览器打开 `http://localhost:10086`。但生产模式仍然要求 https，所以这种自测要把编排改成：

```yaml
  DEV_MODE: "true"
  PUBLIC_ORIGIN: "http://localhost:10086"
```

`DEV_MODE=true` 只允许 loopback 来源、Cookie 不带 Secure，**仅供本机自测，测完改回去**。

### 先确认容器真的起来了

```sh
docker ps --filter name=papergrade
docker logs papergrade-api --tail 20
curl -s http://127.0.0.1:10086/readyz
```

`readyz` 返回 `{"status":"ready","storage_mode":"sqlite"}` 说明服务正常，问题只在前端访问方式。
