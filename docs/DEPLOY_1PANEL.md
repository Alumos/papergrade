# 1Panel 部署步骤

编排文件：[deploy/compose.1panel.yaml](<../deploy/compose.1panel.yaml>)。

## 1. 先替换 5 处

```sh
openssl rand -hex 32    # 生成随机密码/密钥
```

| 位置 | 改成 |
|---|---|
| `DATABASE_URL` 里的 `CHANGE_ME_DB_PASSWORD` | 随机数据库密码 |
| `POSTGRES_PASSWORD` | **与上面完全相同的值** |
| `APP_ENCRYPTION_KEY` | `openssl rand -hex 32` 的输出 |
| `ADMIN_PASSWORD` | 你的登录密码，至少 12 位 |
| `PUBLIC_ORIGIN` | 实际访问的 `https://域名`，结尾不带斜杠 |

`APP_ENCRYPTION_KEY` 一旦启用就**不能再改**：它解密后台保存的模型密钥，换了以后已存密钥全部读不出来。请单独抄一份备份。

## 2. 粘贴部署

1Panel → 容器 → 编排 → 创建编排，名称 `papergrade`，粘贴 YAML，确认部署。

启动顺序：postgres → migrate 建表 → api / worker → gateway。`migrate` 跑完会显示「已退出」，这是正常的，不要给它设自动重启。

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

保存后 API 与 Worker 立即生效，无需重启。模型地址与密钥加密存在数据库，不写进 compose。

## 5. 验收

```sh
docker logs papergrade-api --tail 30
docker logs papergrade-worker --tail 30
curl -s http://127.0.0.1:18080/readyz
```

浏览器打开域名 → 登录 → 建班级（学号位数统一，英语卷要求 4 位数字）→ 传试卷 → 解析 → 采纳 → 发布 → 打印答题卡。

## 6. 备份与升级

必须备份三样：`pgdata` 卷、`evidence` 卷、`APP_ENCRYPTION_KEY`。

升级：把两处镜像标签换成新版本号重新部署即可，**不要** `down -v`。