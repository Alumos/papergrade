# 1Panel 部署步骤

编排文件：[deploy/compose.1panel.yaml](<../deploy/compose.1panel.yaml>)。

## 1. 先改 4 个值

```sh
openssl rand -hex 32   # 用来生成下面的密码
```

| 位置 | 改成 |
|---|---|
| `DATABASE_URL` 里的 `CHANGE_ME_DB_PASSWORD` | 随机数据库密码 |
| `POSTGRES_PASSWORD` | **与上面完全相同** |
| `APP_ENCRYPTION_KEY` | `openssl rand -hex 32` 的输出 |
| `ADMIN_PASSWORD` | 你的登录密码（≥12 位） |
| `PUBLIC_ORIGIN` | 你实际访问的 `https://域名`，结尾不带斜杠 |

`APP_ENCRYPTION_KEY` 一旦用上就**不能改**：它负责解密后台保存的模型密钥，换了以后已存的密钥全部读不出来。请单独备份。

## 2. 粘贴部署

1Panel → 容器 → 编排 → 创建编排，把 YAML 全文粘进去，名称填 `papergrade`，部署。

首次启动顺序是 postgres → migrate 建表 → api/worker → gateway，`migrate` 跑完会显示「已退出」，这是正常的。

## 3. 配反向代理

1Panel → 网站 → 创建网站（绑定你的域名，开 HTTPS），然后在该网站里加反向代理：

- 代理地址：`http://127.0.0.1:18080`
- 发送域名：`$host`

如果 1Panel 的 OpenResty 在容器里、访问不到 `127.0.0.1:18080`，把 compose 里 gateway 的端口映射改成 `"18080:8090"`，代理地址改用宿主机内网 IP。

> 反向代理必须保留原始 `Host` 与 `Origin`。应用会校验 Origin 是否等于 `PUBLIC_ORIGIN`，不一致会返回 403 `CSRF_ORIGIN`。

## 4. 首次登录与模型配置

用 `ADMIN_USERNAME` / `ADMIN_PASSWORD` 登录，进入「模型与服务设置」：

- OCR：类型选 Ollama，地址填你的 Ollama 基址（不带 `/api/chat`），模型填 NAS 上实际的模型名；原生 Ollama 通常没有密钥，留空。
- AI：填 OpenAI 兼容基址（一般以 `/v1` 结尾）、模型名、密钥。
- 保存后 API 与 Worker 立即生效，不用重启。

模型地址与密钥存在数据库里并加密，不写在 compose 里。

## 5. 验收

```sh
docker logs papergrade-api --tail 20
docker logs papergrade-worker --tail 20
```

浏览器打开域名 → 登录 → 建班级（学号必须统一位数，英语卷要求 4 位数字）→ 传试卷 → 解析 → 采纳 → 发布 → 打印答题卡。

## 6. 备份

必须备份三样：`pgdata` 卷、`evidence` 卷、`APP_ENCRYPTION_KEY`。缺主密钥等于模型配置作废。

升级方式：把两处镜像标签改成新版本号，重新部署；不要 `down -v`。