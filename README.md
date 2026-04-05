# CDN Manage

一个前后端分离的 CDN 管理平台。

当前仓库结构：

- `backend/`：Go + Gin + GORM + MySQL + Redis
- `frontend/`：React + TypeScript + Vite + Ant Design
- `.codex/specs/`：需求、设计、任务拆解文档

## 当前进度

截至当前工作区，项目进度已经落到以下阶段：

- 后端：
  已完成基础工程、数据库模型、认证、RBAC、项目与用户管理、存储桶操作、CDN 刷新与资源同步、统一审计记录器、平台级与项目级审计查询接口。
- 前端：
  已完成基础工程骨架、路由骨架、API 请求层、Zustand 状态骨架，以及 `Light` / `Auth` / `Dark` 三套主题与切换机制。
- 当前下一步重点：
  `10.3` 登录页、初始化提示页与权限感知主布局；
  `10.4` 主题切换与路由守卫测试；
  然后进入 `11.x` 和 `12.x` 的业务页面实现。

如果你要确认更细的任务状态，请看：

- [tasks.md](/mnt/e/code/cdnManage/.codex/specs/cdn-management-platform/tasks.md)

## 1. 环境要求

### 后端

- Go 1.24+ 或兼容版本
- MySQL 8.x
- Redis 7.x

### 前端

- Node.js 24+
- npm 11+

## 2. 目录说明

```text
.
├── backend
│   ├── cmd/server/main.go
│   └── internal/...
├── frontend
│   ├── src/
│   └── package.json
└── .codex/specs/
```

## 3. 后端如何启动

### 3.1 配置文件

后端现在使用独立 YAML 配置文件，不再依赖环境变量传业务配置。

配置入口：

- [config.go](/mnt/e/code/cdnManage/backend/internal/config/config.go)
- [loader.go](/mnt/e/code/cdnManage/backend/internal/infra/configloader/loader.go)
- [config.example.yaml](/mnt/e/code/cdnManage/backend/config.example.yaml)

加载规则：

1. 如果当前工作目录下有 `config.yaml`，优先读取它
2. 否则尝试读取 `backend/config.yaml`

推荐做法：

```bash
cp backend/config.example.yaml backend/config.yaml
```

然后按你的环境修改 `backend/config.yaml`

配置示例：

```yaml
server:
  port: 8080

mysql:
  host: 127.0.0.1
  port: 3306
  user: root
  password: 123456
  database: cdn_manage
  max_open_conns: 20
  max_idle_conns: 5

redis:
  host: 127.0.0.1
  port: 6379
  password: ""

jwt:
  secret: replace-with-strong-secret
  issuer: cdn-management-platform
  lifespan_seconds: 3600

session:
  secret: replace-with-session-secret

encryption:
  key: replace-with-32-byte-key

super_admin:
  email: admin@example.com
  password: ChangeMe123!

request_limit: 100
```

### 3.2 Linux/macOS 启动示例

```bash
cd backend
cp config.example.yaml config.yaml
# 修改 config.yaml
GOPROXY=https://goproxy.cn,direct go run ./cmd/server
```

### 3.3 Windows PowerShell 启动示例

```powershell
cd backend
Copy-Item config.example.yaml config.yaml
# 修改 config.yaml
$env:GOPROXY="https://goproxy.cn,direct"
go run ./cmd/server
```

### 3.4 启动后行为

- 应用入口：[`backend/cmd/server/main.go`](/mnt/e/code/cdnManage/backend/cmd/server/main.go)
- 启动时会自动连接 MySQL 和 Redis
- 启动时会自动执行数据库迁移
- 如果用户表为空，会自动创建超级管理员账号

### 3.5 后端验证

健康检查接口：

```bash
curl http://127.0.0.1:8080/health
```

## 4. 前端如何启动

### 4.1 安装依赖

建议使用国内镜像：

```bash
cd frontend
npm_config_registry=https://registry.npmmirror.com npm install
```

### 4.2 前端配置如何修改

前端当前只有一个显式 API 配置：

- [client.ts](/mnt/e/code/cdnManage/frontend/src/services/api/client.ts)

读取变量：

- `VITE_API_BASE_URL`

默认值：

- `/api/v1`

这意味着：

- 生产环境如果前后端同域部署，可以不配
- 本地前后端分开跑时，建议显式指定后端地址

### 4.3 本地开发推荐配置

在 `frontend/` 下新建 `.env.development`：

```env
VITE_API_BASE_URL=http://127.0.0.1:8080/api/v1
```

然后启动：

```bash
cd frontend
npm run dev
```

默认访问地址通常是：

```text
http://127.0.0.1:5173
```

### 4.4 前端生产构建

```bash
cd frontend
npm run build
```

构建产物在：

```text
frontend/dist
```

## 5. 前后端一起运行

推荐顺序：

1. 先启动 MySQL
2. 再启动 Redis
3. 启动后端 `backend`
4. 启动前端 `frontend`

本地常用组合：

- 后端：`http://127.0.0.1:8080`
- 前端：`http://127.0.0.1:5173`
- 前端 `.env.development`：`VITE_API_BASE_URL=http://127.0.0.1:8080/api/v1`

## 6. 后端路由如何配置

后端路由总装配入口：

- [server.go](/mnt/e/code/cdnManage/backend/internal/transport/server.go)

### 6.1 认证

前缀：`/api/v1/auth`

- `POST /api/v1/auth/login`
- `GET /api/v1/auth/me`
- `POST /api/v1/auth/change-password`

定义位置：

- [auth.go](/mnt/e/code/cdnManage/backend/internal/handler/auth/auth.go)

### 6.2 用户管理

前缀：`/api/v1/users`

- `GET /api/v1/users`
- `POST /api/v1/users`
- `PUT /api/v1/users/:id`
- `DELETE /api/v1/users/:id`
- `PUT /api/v1/users/:id/project-bindings`

定义位置：

- [users.go](/mnt/e/code/cdnManage/backend/internal/handler/users/users.go)

### 6.3 项目与 CDN

前缀：`/api/v1/projects`

- `GET /api/v1/projects`
- `POST /api/v1/projects`
- `GET /api/v1/projects/:id`
- `PUT /api/v1/projects/:id`
- `DELETE /api/v1/projects/:id`
- `GET /api/v1/projects/:id/cdns`
- `PUT /api/v1/projects/:id/cdns`
- `POST /api/v1/projects/:id/cdns/refresh-url`
- `POST /api/v1/projects/:id/cdns/refresh-directory`
- `POST /api/v1/projects/:id/cdns/sync`

定义位置：

- [projects.go](/mnt/e/code/cdnManage/backend/internal/handler/projects/projects.go)

### 6.4 存储管理

前缀：`/api/v1/storage` 和 `/api/v1/projects/:id/storage`

- `POST /api/v1/storage/connections/validate`
- `GET /api/v1/projects/:id/storage/objects`
- `GET /api/v1/projects/:id/storage/download`
- `GET /api/v1/projects/:id/storage/audits`
- `POST /api/v1/projects/:id/storage/upload`
- `DELETE /api/v1/projects/:id/storage/objects`
- `PUT /api/v1/projects/:id/storage/rename`

定义位置：

- [storage.go](/mnt/e/code/cdnManage/backend/internal/handler/storage/storage.go)

### 6.5 审计查询

前缀：`/api/v1/audits` 和 `/api/v1/projects/:id/audits`

- `GET /api/v1/audits`
- `GET /api/v1/projects/:id/audits`

定义位置：

- [audits.go](/mnt/e/code/cdnManage/backend/internal/handler/audits/audits.go)

## 7. 前端路由如何配置

前端路由入口：

- [index.tsx](/mnt/e/code/cdnManage/frontend/src/routes/index.tsx)

当前已配置的页面路由：

- `/`
- `/login`
- `/setup`
- `/unauthorized`
- `/projects`
- `/users`
- `/storage`
- `/cdn`
- `/audits`

当前这些页面大多还是骨架或占位页，后续任务会继续填充。

## 8. 主题如何配置

主题入口：

- [themes.ts](/mnt/e/code/cdnManage/frontend/src/app/themes.ts)
- [AppProviders.tsx](/mnt/e/code/cdnManage/frontend/src/app/AppProviders.tsx)
- [shell.ts](/mnt/e/code/cdnManage/frontend/src/store/shell.ts)

当前支持：

- `light`
- `auth`
- `dark`

规则：

- `/login` 和 `/setup` 会强制使用 `auth` 主题
- 主布局页可通过顶部切换控件切换 `light` 和 `dark`

## 9. 当前已知注意事项

### 9.1 前端开发模式没有配置 Vite 代理

当前 [vite.config.ts](/mnt/e/code/cdnManage/frontend/vite.config.ts) 没有 `server.proxy`。

所以本地开发时：

- 如果不配置 `VITE_API_BASE_URL`
- 前端请求 `/api/v1/...` 会打到 `5173` 自己

建议直接使用：

```env
VITE_API_BASE_URL=http://127.0.0.1:8080/api/v1
```

### 9.2 前端构建体积较大

`npm run build` 当前能通过，但已有 chunk size warning。后续做页面懒加载或分包时再优化。

### 9.3 后端配置文件不要提交真实密钥

建议：

- 把真实配置写进 `backend/config.yaml`
- 仓库中只保留 `backend/config.example.yaml`
- 不要把生产密钥和密码提交到 Git

## 10. 常用命令

### 后端

```bash
cd backend
GOPROXY=https://goproxy.cn,direct go test ./...
GOPROXY=https://goproxy.cn,direct go run ./cmd/server
```

### 前端

```bash
cd frontend
npm_config_registry=https://registry.npmmirror.com npm install
npm run dev
npm run build
```
