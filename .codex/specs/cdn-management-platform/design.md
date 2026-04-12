# Design Document

## Overview

CDN 管理平台采用前后端分离架构，目标是在单一平台内统一管理多云对象存储、项目级 CDN 配置、权限模型与审计追踪。系统面向电脑浏览器访问，前端使用 React + TypeScript + Ant Design，后端使用 Go + Gin + GORM，数据层使用 MySQL + Redis。

本设计优先满足以下目标：

- 通过统一抽象屏蔽云厂商对象存储与 CDN 刷新的差异。
- 通过项目作用域与 RBAC 组合校验防止越权访问。
- 通过审计日志确保所有关键写操作与外部云操作可追溯。
- 通过可扩展的 Provider 架构支持后续新增云厂商。
- 通过三套主题系统满足 Light、Auth、Dark 的桌面端体验要求。

### Key Design Decisions

1. 采用单体分层架构，而不是拆分微服务。
   - 理由：当前功能边界清晰但业务规模尚小，单体更利于快速迭代、统一权限校验与审计闭环。
2. 将对象存储与 CDN 能力抽象为 Provider 接口层。
   - 理由：四大云厂商在认证、路径、刷新 API 和错误模型上存在差异，抽象层可稳定上层业务逻辑。
3. 将审计写入作为服务层标准流程，而不是仅在控制器层记录。
   - 理由：服务层更接近业务动作边界，能覆盖 API、后台任务和后续 CLI/脚本调用。
4. 将 Redis 用于会话、权限缓存与短时任务状态，而不是存储核心业务真值。
   - 理由：核心配置、授权与审计必须可持久化、可审计、可回放，应以 MySQL 为准。

### Research Findings Incorporated

- Ant Design v5 支持通过 `ConfigProvider.theme` 动态切换 Design Token，适合实现平台级 Light、Auth、Dark 三主题切换。
- Gin 支持全局中间件、路由组中间件与嵌套路由组，适合按“认证 -> RBAC -> 项目作用域 -> 审计”顺序组织请求管线。
- Gin 的错误处理中间件模式适合统一输出结构化 JSON 错误响应。
- GORM 支持关联、`Preload` 与事务，适合项目授权关系、项目配置关系与关键写操作的一致性处理。
- Redis 官方 Go 客户端为 `go-redis`，适合承接缓存与会话能力。

## Architecture

### High-Level Architecture

```mermaid
flowchart LR
    Browser[Desktop Browser<br/>React + TypeScript + Ant Design]
    API[Gin API Server]
    Auth[Auth & RBAC Middleware]
    App[Application Services]
    Provider[Cloud Provider Adapters]
    MySQL[(MySQL)]
    Redis[(Redis)]
    OSS[Object Storage APIs]
    CDN[CDN Provider APIs]

    Browser --> API
    API --> Auth
    Auth --> App
    App --> MySQL
    App --> Redis
    App --> Provider
    Provider --> OSS
    Provider --> CDN
```

### Backend Layering

后端按以下层级组织：

1. `handler` 层
   - 负责 HTTP 参数绑定、响应编码、调用应用服务。
2. `middleware` 层
   - 负责认证、角色校验、项目作用域校验、错误转换、请求追踪、审计上下文注入。
3. `service` 层
   - 负责业务规则、事务边界、跨仓储协调、审计记录触发、Provider 调用编排。
4. `repository` 层
   - 负责 GORM 查询与持久化。
5. `provider` 层
   - 负责封装各云厂商对象存储与 CDN 刷新能力。
6. `infra` 层
   - 负责数据库、Redis、加密、日志、配置加载等基础设施。

### Frontend Architecture

前端采用 React 单页应用，按领域拆分模块：

- `auth`
  - 登录、首次初始化提示、修改密码。
- `projects`
  - 项目列表、项目详情、项目配置编辑。
- `storage`
  - 存储桶层级文件浏览、上传、下载、删除、重命名、审计查询入口。
- `cdn`
  - CDN 地址管理、目录刷新、URL 刷新、资源同步。
- `users`
  - 用户管理、项目角色绑定。
- `audit`
  - 平台审计查询、项目范围审计查询。
- `layout`
  - 导航、面包屑、主题切换、权限感知菜单。

### Theme Strategy

前端基于 Ant Design v5 的 Token 体系实现三套主题：

- `Light`
  - 默认工作台主题，强调清晰信息密度。
- `Auth`
  - 登录与初始化页面专用主题，强化品牌与身份验证氛围。
- `Dark`
  - 暗色工作台主题，用于低亮度环境。

赛博科技风格通过以下方式控制，不影响可读性：

- 以蓝青色系作为强调色，而不是大面积高饱和背景。
- 在登录页、初始化页与空状态页加入轻量渐变和网格纹理。
- 在业务表格页保持高对比与高信息密度，避免过度装饰。

## Components and Interfaces

### Backend Core Components

#### 1. Authentication Component

职责：

- 用户登录
- 密码校验与密码修改
- 首次启动超级管理员初始化
- 会话或令牌签发与校验

主要接口：

- `POST /api/v1/auth/login`
- `POST /api/v1/auth/change-password`
- `GET /api/v1/auth/me`

#### 2. User and RBAC Component

职责：

- 用户创建、编辑、禁用、删除
- 平台管理员重置用户密码
- 项目角色绑定
- 平台管理员与项目角色校验

主要接口：

- `GET /api/v1/users`
- `POST /api/v1/users`
- `PUT /api/v1/users/:id`
- `DELETE /api/v1/users/:id`
- `PUT /api/v1/users/:id/password`
- `GET /api/v1/users/:id/project-bindings`
- `PUT /api/v1/users/:id/project-bindings`

RBAC 规则：

- 平台管理员：全平台资源可管理。
- 项目管理员：仅可管理已授权项目的资源、CDN、审计查询。
- 项目只读用户：仅可查看已授权项目的配置、资源与日志。

用户绑定管理交互策略：

- 绑定管理弹框打开时，前端先请求 `GET /api/v1/users/:id/project-bindings` 获取用户项目绑定快照。
- 前端将快照结果预填到绑定编辑区，避免平台管理员重复录入已存在绑定。
- 绑定编辑区采用紧凑绑定表格，而非多卡片堆叠，单行包含：项目选择、角色选择、删除操作。
- 保存操作继续调用 `PUT /api/v1/users/:id/project-bindings`，保持“提交即替换全部绑定”的后端语义不变。
- 保存成功后前端刷新快照或以返回结果回填，保证二次打开弹框展示最近一次保存状态。

#### 3. Project Management Component

职责：

- 项目创建、编辑、删除
- 项目绑定云厂商、存储桶、CDN 地址
- 项目隔离校验
- 项目内多云混合绑定规则校验
- 项目编辑场景下的绑定项凭据保留与替换编排

主要接口：

- `GET /api/v1/projects`
- `POST /api/v1/projects`
- `GET /api/v1/projects/:id`
- `PUT /api/v1/projects/:id`
- `DELETE /api/v1/projects/:id`
- `GET /api/v1/projects/accessible`
- `GET /api/v1/projects/:id/context`

多云绑定策略：

- 同一项目允许 `0~2` 个存储桶绑定与 `0~2` 个 CDN 绑定，每个绑定项独立携带 `provider_type`。
- 取消“同一项目内绑定项 provider_type 必须一致”的校验约束。
- 保留每类绑定“存在绑定时恰好一个 Primary”的约束，Primary 语义限定在同类绑定范围内而非按云厂商分组。
- 存储与 CDN 调用以“目标绑定项”作为路由入口，根据该绑定项的 `provider_type` 解析并调用对应 Provider。

项目编辑凭据策略：

- 绑定项请求新增 `credentialOperation` 字段，支持 `KEEP` 与 `REPLACE`。
- 绑定项有 `id` 且未传 `credentialOperation` 时按 `KEEP` 处理，以兼容历史前端提交。
- `KEEP` 模式复用数据库已加密凭据，不要求重新输入 AK/SK。
- `REPLACE` 模式要求提交完整凭据字段并按厂商规则校验后覆盖历史密文。
- 当已有绑定项 `providerType` 发生变化时强制 `REPLACE`，禁止以 `KEEP` 提交。
- 新增绑定项必须使用 `REPLACE`，避免创建无凭据绑定。

CDN 绑定字段简化策略（软废弃阶段）：

- `cdnEndpoint` 作为绑定核心标识保留，前端展示名称统一为“CDN 域名”。
- `region` 在 CDN 绑定表单中改为可选字段；后端不再对该字段做必填约束。
- `purgeScope` 从“CDN 绑定配置输入”中移除，刷新语义改由操作接口 `refresh-url` 与 `refresh-directory` 决定。
- 在软废弃阶段保留 `project_cdns.purge_scope` 字段以兼容历史数据读取与回滚安全，不作为新配置请求的必需输入。
- 后续硬删除阶段再通过迁移移除 `project_cdns.purge_scope` 与相关兼容代码。

项目可见性与上下文策略：

- 保留 `GET /api/v1/projects` 与 `GET /api/v1/projects/:id` 作为平台管理员管理视图接口。
- 新增 `GET /api/v1/projects/accessible` 作为“当前用户可见项目列表”接口：
  - 平台管理员返回全部项目。
  - 标准用户返回其已绑定项目角色的项目。
- 新增 `GET /api/v1/projects/:id/context` 作为“项目上下文”接口，至少返回该项目可见的 `buckets` 与 `cdns`，并受项目作用域中间件保护。
- 存储页与 CDN 操作页优先使用 `accessible + context` 组合加载，不依赖平台管理员管理接口。
- 写操作按钮状态由“当前项目上下文下的项目角色可写性”决定，而不是仅依据平台角色决定。

#### 4. Storage Component

职责：

- 存储桶连接校验
- 分层目录文件列表查询与分页
- 上传、下载、单文件删除、目录递归删除、批量删除、重命名
- 自动识别云厂商
- 支持阿里云 OSS 与腾讯云 COS 等多云对象存储绑定并行使用
- 压缩包上传会话跟踪与汇总
- 上传阶段 A/阶段 B 的进度采集与展示支撑
- 压缩包条目并发上传编排与失败汇总
- 目录递归删除编排与批量删除逐项结果汇总

主要接口：

- `POST /api/v1/storage/connections/validate`
- `GET /api/v1/projects/:id/storage/objects`
- `GET /api/v1/projects/:id/storage/objects/tree`
- `POST /api/v1/projects/:id/storage/upload`
- `GET /api/v1/projects/:id/storage/upload-sessions`
- `GET /api/v1/projects/:id/storage/upload-sessions/:sessionId`
- `GET /api/v1/projects/:id/storage/download`
- `DELETE /api/v1/projects/:id/storage/objects`
- `DELETE /api/v1/projects/:id/storage/objects/batch`
- `PUT /api/v1/projects/:id/storage/rename`

Provider 抽象建议：

```go
type ObjectStorageProvider interface {
    Detect(ctx context.Context, credential CredentialPayload, bucket string) (ProviderType, error)
    ListObjects(ctx context.Context, req ListObjectsRequest) ([]ObjectInfo, error)
    UploadObject(ctx context.Context, req UploadObjectRequest) error
    DownloadObject(ctx context.Context, req DownloadObjectRequest) (io.ReadCloser, ObjectMeta, error)
    DeleteObject(ctx context.Context, req DeleteObjectRequest) error
    RenameObject(ctx context.Context, req RenameObjectRequest) error
}
```

目录删除策略：

- 目录在对象存储中按 `prefix/` 逻辑目录处理，不要求 Provider 暴露独立“删除目录”原语。
- 单目录删除复用 `DELETE /api/v1/projects/:id/storage/objects`，当 `key` 以 `/` 结尾时服务层切换为递归删除模式。
- 递归删除流程按目录前缀分页列出对象，再逐对象调用 Provider 删除，直到该前缀下对象被清空。
- 批量删除接口允许同时提交文件 key 与目录 key；目录 key 在同一编排中按递归删除模式执行。
- 批量删除结果项需区分 `targetType=file|directory`，目录项额外返回 `deletedObjects`、`failedObjects` 与失败摘要。
- 若目录前缀下未找到任何对象，系统返回成功结果并将 `deletedObjects` 记为 `0`，避免把空目录清理视为异常。

前端交互约束：

- 目录行保留“删除”入口，但确认文案需明确说明会删除目录下全部对象。
- 单文件行保留下载、重命名、删除入口。
- 批量选择允许同时选择文件与目录，确认弹窗需提示“目录将递归删除其下全部对象”。
- 删除完成后前端刷新当前目录列表并清空选中项；若当前目录本身被删除，前端返回上一级目录后重新查询。
- 目录重命名入口保持可用，确认文案需明确说明会迁移目录前缀下全部对象。

上传性能与进度策略：

- 上传并发策略
  - 后端在压缩包条目上传链路使用 worker pool 并发执行对象上传。
  - 并发度使用后端配置项 `upload.archive_parallelism` 控制，默认值固定为 `4`，支持在 `2~8` 范围内调整。
  - 会话统计在并发执行下保持一致，包含 `totalEntries`、`successEntries`、`failedEntries`、`skipped`。
- 两段进度策略
  - 阶段 A（浏览器到后端）：前端通过 `onUploadProgress` 展示请求体传输进度。
  - 阶段 B（后端处理）：前端基于上传响应中的 `sessionId` 轮询会话汇总审计，展示处理进度与失败摘要。
  - 阶段 B 的数据来源为 `object.upload_archive` 汇总审计与 `object.upload` 明细审计关联视图。
- `key` 与 `keyPrefix` 语义约束
  - 单文件上传仅使用 `key` 指定目标对象路径。
  - 多文件与压缩包上传使用 `keyPrefix` 作为统一前缀，并保留各文件相对路径。
  - 前端表单按上传模式展示对应说明，避免单文件与多文件语义混淆。

上传并发配置示例：

```yaml
upload:
  max_file_size_mb: 20
  archive_parallelism: 4
```

配置约束：

- `archive_parallelism` 未配置时使用默认值 `4`。
- `archive_parallelism` 小于 `2` 时按 `2` 处理，大于 `8` 时按 `8` 处理。
- 配置解析与默认值逻辑在 `backend/internal/config/config.go` 统一处理，并在 `backend/config.example.yaml` 提供示例。

#### 5. CDN Component

职责：

- 项目 CDN 地址配置管理
- URL 刷新与目录刷新
- 资源同步编排
- 支持阿里云 CDN 与腾讯云 CDN 等多云 CDN 绑定并行使用

主要接口：

- `POST /api/v1/projects/:id/cdn/refresh-url`
- `POST /api/v1/projects/:id/cdn/refresh-directory`
- `POST /api/v1/projects/:id/cdn/sync`
- `GET /api/v1/projects/:id/cdns/directories`

Provider 抽象建议：

```go
type CDNProvider interface {
    RefreshURLs(ctx context.Context, req RefreshURLsRequest) (ProviderTaskResult, error)
    RefreshDirectories(ctx context.Context, req RefreshDirectoriesRequest) (ProviderTaskResult, error)
    SyncLatestResources(ctx context.Context, req SyncResourcesRequest) (ProviderTaskResult, error)
}
```

说明：

- “资源同步” 在平台层定义为业务流程，而不是依赖所有厂商提供同名原生 API。
- 平台先确定同步目标对象，再触发相应 CDN 刷新，以达成“同步最新资源到 CDN，自动更新缓存”的业务效果。

CDN 操作台页面交互策略（与存储页对齐）：

- 页面结构
  - 顶部提供共享上下文区：项目下拉、CDN 域名下拉、资源同步所需存储桶下拉。
  - 中部提供操作区：`URL 刷新`、`目录刷新`、`资源同步` 三个操作在同一页面内切换执行。
  - 底部提供结果区：统一展示最近一次提交结果与任务状态字段。
- 上下文加载规则
  - 进入页面时先加载当前用户可见项目列表。
  - 选择项目后并行加载该项目的 CDN 绑定与存储桶绑定选项。
  - CDN 下拉默认选中该项目 Primary CDN；资源同步的存储桶下拉默认选中 Primary Bucket。
  - 当项目切换后，重置当前操作输入区并保留结果区历史展示策略。
- 交互约束
  - URL 刷新、目录刷新、资源同步共用项目与 CDN 选择状态，避免三处重复输入。
  - 资源同步额外要求存储桶下拉有值，不再使用手工输入 BucketName。
  - 目录刷新支持“目录查询”辅助能力，用户可先查询目录候选再选择目标目录。
  - `project_read_only` 允许目录查询，但不允许提交 URL 刷新、目录刷新与资源同步写操作。
  - `project_admin` 与平台管理员允许目录查询与提交刷新/同步写操作。
  - 绑定选项为空时展示只读提示，并禁用对应提交按钮，避免产生无效请求。
  - 表单校验错误需定位到具体字段（项目、CDN 域名、存储桶、URL/目录/路径输入）。
- 兼容性与边界
  - 请求载荷字段保持后端接口兼容：继续使用 `cdnEndpoint`、`bucketName`、`urls|directories|paths`。
  - 不改变后端刷新与同步业务语义，仅优化前端输入组织与校验流程。

#### 6. Audit Component

职责：

- 记录关键业务操作
- 支持按项目、用户、操作类型、时间范围筛选
- 按角色限制可见范围
- 支持平台级与项目级审计筛选下拉选项加载与联动查询

主要接口：

- `GET /api/v1/audits`
- `GET /api/v1/projects/:id/audits`
- `GET /api/v1/audits/filter-options`
- `GET /api/v1/projects/:id/audits/filter-options`

审计筛选交互策略：

- 平台级审计查询页
  - `Action` 与 `Target Type` 使用下拉筛选，不再依赖手工输入。
  - 下拉选项由平台级筛选选项接口返回，前端支持可清空与可搜索。
- 项目级审计查询页
  - `Project ID`、`Action`、`Target Type` 使用下拉筛选。
  - `Project ID` 下拉仅展示当前用户可见项目，并展示 `projectId + projectName` 组合标签。
  - 选中项目后，`Action` 与 `Target Type` 支持按项目上下文联动加载。
- 空状态与容错
  - 任一下拉维度无可选值时显示可读提示并保留查询入口。
  - 清空任一下拉值时移除对应查询参数，回退为该维度非限制查询。

审计写入范围：

- 登录成功与失败
- 用户管理操作
- 管理员重置用户密码操作
- 项目管理操作
- 存储桶文件操作
- CDN 刷新操作
- 资源同步操作
- 越权访问拦截
- 敏感配置变更

#### 7. Overview Workspace Component

职责：

- 提供业务工作台风格首页，优先呈现日常操作所需信息
- 汇总上传与 CDN 操作关键指标，支持时间范围切换
- 展示当前用户可见项目范围的项目工作台列表
- 展示最近失败上传与失败 CDN 操作的运维风险摘要
- 展示最近活动时间线并支持按 requestId 追踪

主要接口：

- `GET /api/v1/projects/accessible`
- `GET /api/v1/projects/:id/context`
- `GET /api/v1/audits`
- `GET /api/v1/projects/:id/audits`

页面布局策略：

- 顶部区域：欢迎信息、时间范围切换（默认 `24h`）、手动刷新、快捷操作入口。
- 中部区域：业务核心卡片（上传会话总数、上传失败数、CDN 操作总数、CDN 失败数、可见项目数）。
- 主区域：项目工作台列表，按“最近活跃时间”默认排序，行内提供 `Storage` 与 `CDN` 入口。
- 风险区域：最近失败上传会话与最近失败 CDN 操作，支持带筛选条件跳转。
- 底部区域：最近活动时间线，展示 `actor`、`project`、`action`、`result`、`time`、`requestId`。

权限与可见性策略：

- 平台管理员看到全平台范围汇总与项目列表。
- 项目角色用户仅看到可见项目范围汇总与项目列表。
- `project_read_only` 隐藏写操作快捷入口，保留查询与导航入口。
- 所有统计与风险数据必须遵循项目作用域与角色可见性约束，不得跨项目泄露。

性能策略：

- 第一阶段复用现有 `accessible/context/audits` 接口在前端聚合展示，降低后端改造风险。
- 第二阶段按需要新增聚合接口（如 `/api/v1/overview`）减少前端并发请求数与重复计算。
- 首页自动刷新采用 `30~60` 秒窗口；用户进行手工筛选时暂停自动刷新以避免打断操作。

#### 8. Neo-Tech Theme Foundation

职责：

- 建立统一的 Neo-Tech 视觉语言与主题令牌体系。
- 保证 Light、Auth、Dark 三套主题在视觉风格统一前提下维持信息可读性。
- 约束动效在“可感知反馈”与“性能友好”之间取得平衡。

设计策略：

- 主题令牌采用“语义层 + 组件层”两级结构：
  - 语义层包含主色、成功、告警、背景层级、边框层级、阴影层级。
  - 组件层映射到按钮、表格、表单、卡片、标签等关键组件。
- Light 主题采用浅色科技底纹与冷色高对比前景，避免纯白平铺导致层次不足。
- Auth 主题使用独立背景与卡片样式强化身份验证场景，不改变认证流程行为。
- Dark 主题延续高对比科技风，但与 Light/Auth 保持统一交互状态语义与控件层级。
- 动效采用分级方案：
  - 标准动效：用于卡片进入、按钮悬停、状态切换。
  - 降级动效：在低性能环境或偏好减少动效时自动禁用非关键动画。

#### 9. Internationalization Architecture

职责：

- 提供默认 `zh-CN`、可切换 `en-US` 的统一国际化机制。
- 保证页面文案、导航标签、按钮文本、提示信息具备一致语言渲染行为。
- 确保语言偏好可持久化并在刷新或重登后恢复。

架构策略：

- 前端引入统一 i18n Provider（推荐 `react-i18next`）并在应用入口挂载。
- 语言资源按模块拆分，至少包含 `common`、`nav`、`overview`、`auth`、`audits`、`storage`、`cdn`、`users`、`projects`。
- 默认语言为 `zh-CN`，可通过顶栏切换到 `en-US`。
- 语言选择先写入本地持久化存储，后续可扩展到用户配置服务端同步。
- 缺失键回退策略：
  - 优先回退 `zh-CN`。
  - 记录可追踪告警用于后续补齐翻译键。

### Middleware Pipeline

建议中间件顺序如下：

1. `RequestIDMiddleware`
2. `StructuredLoggerMiddleware`
3. `RecoveryMiddleware`
4. `AuthMiddleware`
5. `RBACMiddleware`
6. `ProjectScopeMiddleware`
7. `AuditContextMiddleware`
8. `ErrorHandlerMiddleware`

设计说明：

- `RBACMiddleware` 负责用户是否具备某类动作权限。
- `ProjectScopeMiddleware` 负责目标项目是否在用户授权范围内。
- 两者拆分后，权限模型更清晰，日志也能区分“角色不足”与“项目越权”。

### Async and Caching Strategy

Redis 用途限定为：

- 登录态或会话缓存
- 用户项目权限快照缓存
- 短时任务状态缓存，例如 CDN 刷新请求状态
- 短时列表缓存，例如项目主页统计信息

不进入 Redis 的数据：

- 用户真值信息
- 项目真值配置
- 存储桶凭证真值
- 审计日志真值

资源同步与刷新可采用“请求立即返回任务受理结果 + 前端轮询状态”的方式，避免长请求阻塞。

## Data Models

### Entity Model

```mermaid
erDiagram
    USERS ||--o{ USER_PROJECT_ROLES : has
    PROJECTS ||--o{ USER_PROJECT_ROLES : has
    PROJECTS ||--o{ PROJECT_BUCKETS : has
    PROJECTS ||--o{ PROJECT_CDNS : has
    PROJECTS ||--o{ AUDIT_LOGS : has
    USERS ||--o{ AUDIT_LOGS : creates

    USERS {
        bigint id
        string username
        string email
        string password_hash
        string status
        string platform_role
        datetime created_at
        datetime updated_at
    }

    PROJECTS {
        bigint id
        string name
        string description
        datetime created_at
        datetime updated_at
    }

    USER_PROJECT_ROLES {
        bigint id
        bigint user_id
        bigint project_id
        string project_role
        datetime created_at
    }

    PROJECT_BUCKETS {
        bigint id
        bigint project_id
        string provider_type
        string bucket_name
        string region
        text credential_ciphertext
        boolean is_primary
        datetime created_at
        datetime updated_at
    }

    PROJECT_CDNS {
        bigint id
        bigint project_id
        string provider_type
        string cdn_endpoint
        string purge_scope
        boolean is_primary
        datetime created_at
        datetime updated_at
    }

    AUDIT_LOGS {
        bigint id
        bigint actor_user_id
        bigint project_id
        string action
        string target_type
        string target_identifier
        string result
        string request_id
        json metadata
        datetime created_at
    }
```

### Model Details

#### `users`

- `platform_role`
  - 枚举：`super_admin`、`platform_admin`、`standard_user`
- `status`
  - 枚举：`active`、`disabled`
- `password_hash`
  - 使用密码摘要算法存储，不保存明文密码

#### `projects`

- 保存项目名称、描述、创建时间
- 不直接内嵌云配置，避免项目主表膨胀

#### `user_project_roles`

- 表达“一个用户可拥有多个项目角色”的多对多关系
- 建议唯一约束：`(user_id, project_id)`

#### `project_buckets`

- 每项目允许 1 至 2 条有效绑定
- `credential_ciphertext` 存储加密后的敏感凭据
- `provider_type` 用于落库记录识别后的厂商类型

#### `project_cdns`

- 每项目允许 1 至 2 条有效绑定
- 保留 `provider_type`，便于后续支持厂商差异化刷新参数

#### `audit_logs`

- `action`
  - 例如：`object.upload`、`object.delete`、`cdn.refresh_url`、`user.disable`
- `result`
  - 枚举：`success`、`failure`、`denied`
- `metadata`
  - 保存请求摘要、错误码、对象路径、刷新路径、IP、User-Agent 等补充信息
  - 对目录递归删除与批量删除补充 `recursive`、`targetType`、`deletedObjects`、`failedObjects`、`batch`

#### `upload_session`（基于审计汇总的逻辑模型）

- 该模型用于表达一次压缩包上传会话的聚合信息。
- 初版不新增独立数据库表，基于审计日志中的会话标识与元数据聚合得到。
- 建议字段：
  - `session_id`
  - `project_id`
  - `actor_user_id`
  - `archive_name`
  - `started_at`
  - `finished_at`
  - `duration_ms`
  - `total_entries`
  - `success_entries`
  - `failed_entries`
  - `status`

#### `storage_tree_entry`（接口响应逻辑模型）

- 用于表达当前层级目录中的条目。
- 建议字段：
  - `name`
  - `path`
  - `type`（`file` 或 `directory`）
  - `size`
  - `last_modified`
  - `content_type`

#### `delete_result`（接口响应逻辑模型）

- 用于表达单文件删除、目录递归删除和批量删除中的逐项结果。
- 建议字段：
  - `key`
  - `target_type`（`file` 或 `directory`）
  - `result`（`success` 或 `failure`）
  - `deleted_objects`
  - `failed_objects`
  - `error_code`
  - `reason`

### Security and Secret Management

敏感数据处理：

- 用户密码使用密码摘要存储。
- AccessKey 与关联敏感凭据使用应用级加密后落库。
- 后端通过配置注入主加密密钥，不写入源码仓库。
- API 响应默认脱敏，前端不回显完整凭据。

权限控制：

- 控制器层不直接信任前端传入的项目 ID。
- 所有项目级查询与写操作均经过项目作用域校验。
- 删除、重命名、刷新、同步均要求写权限。
- 查看列表、下载、审计查询按角色细分只读与管理权限。

### Initialization Strategy

系统启动时执行初始化检查：

1. 检查 `users` 表是否为空。
2. 若为空，则创建超级管理员账号。
3. 生成初始登录标识与一次性初始密码或从环境变量读取初始化密码。
4. 记录初始化审计日志。

推断说明：
基于你的运维背景和安全要求，首版设计倾向于“环境变量提供初始密码或一次性密码”，这样比硬编码默认密码更安全。

## Error Handling

### Error Categories

- `AUTHENTICATION_FAILED`
  - 登录失败、会话失效、身份凭证缺失
- `AUTHORIZATION_DENIED`
  - 角色不足
- `PROJECT_SCOPE_DENIED`
  - 访问未授权项目
- `VALIDATION_ERROR`
  - 请求参数不合法
- `PASSWORD_POLICY_VIOLATION`
  - 新密码不满足密码策略要求
- `PROVIDER_CONNECTION_FAILED`
  - 存储桶连接校验失败
- `PROVIDER_OPERATION_FAILED`
  - 对象存储或 CDN 厂商操作失败
- `PROVIDER_NOT_REGISTERED`
  - 绑定项使用的云厂商类型未在当前服务实例注册
- `PROVIDER_CHANGE_REQUIRES_CREDENTIAL_REPLACE`
  - 已有绑定项变更云厂商类型时未提交凭据替换
- `CREDENTIAL_NOT_FOUND_FOR_KEEP`
  - 绑定项使用 KEEP 模式但历史凭据不可用
- `UPLOAD_ARCHIVE_PROCESSING_FAILED`
  - 压缩包解析或条目处理失败
- `INVALID_BATCH_OPERATION`
  - 批量操作请求参数不合法或目标类型不支持
- `RESOURCE_NOT_FOUND`
  - 项目、用户、对象、绑定配置不存在
- `CONFLICT_ERROR`
  - 用户、项目、绑定关系冲突
- `INTERNAL_ERROR`
  - 未分类内部错误

### Error Response Format

统一 JSON 错误结构：

```json
{
  "code": "AUTHORIZATION_DENIED",
  "message": "project write permission required",
  "requestId": "req_123456",
  "details": {}
}
```

### Error Handling Rules

- 外部厂商错误在 `provider` 层转换为统一领域错误。
- `handler` 层不直接透传厂商原始错误正文，避免泄露敏感信息。
- 所有失败写操作必须记录审计日志。
- 越权与认证失败要分别记录，便于审计分析。
- 文件上传与同步失败时，响应中返回可追踪请求编号。
- 压缩包上传失败时，响应中返回上传会话标识与失败条目摘要。
- 目录递归删除出现部分失败时，响应中必须返回已删除对象数、失败对象数与失败摘要。
- 批量删除返回逐项结果时，必须区分文件与目录、成功与失败及失败原因。
- 目录重命名出现部分失败时，响应中必须返回已迁移对象数、失败对象数与失败摘要。
- 创建或更新项目绑定时，若绑定项 provider 未注册，响应必须返回可定位到具体绑定项的错误详情。
- 已有绑定项变更 provider 且凭据操作模式为 `KEEP` 时，响应必须返回可定位绑定项的约束错误详情。
- 绑定项以 `KEEP` 提交但无可用历史凭据时，响应必须返回可定位绑定项的凭据缺失错误详情。
- 刷新操作语义由刷新接口类型决定，不依赖绑定配置中的 `purgeScope` 值。
- CDN 目录查询接口必须执行项目作用域校验与项目读权限校验，且在缺少 `projectId` 或 `bucketName` 时返回字段级参数错误。
- 用户项目绑定快照接口必须限制为平台管理员访问，且在目标用户不存在时返回可追踪的 `user_not_found` 错误。
- 可见项目列表与项目上下文接口在权限拒绝时必须返回可追踪错误码，并在 `details` 中包含 `projectID` 或权限判定关键信息。
- Overview 首页的统计与风险摘要在返回数据时必须遵循用户可见项目范围，且在跳转参数中保持筛选条件一致性。

## Testing Strategy

### Backend Testing

重点测试以下核心逻辑：

- RBAC 判定与项目作用域校验
- 管理员重置用户密码权限与密码策略校验
- 项目绑定数量约束
- 首次启动超级管理员初始化逻辑
- Provider 抽象层的参数组装与错误映射
- 多云混合绑定校验逻辑与 provider 路由逻辑
- 绑定项 `KEEP`/`REPLACE` 凭据操作策略与 provider 变更约束
- CDN 绑定 `region` 可选输入与 `purgeScope` 软废弃兼容策略
- 审计日志写入触发条件
- 存储桶文件操作服务逻辑
- 层级目录列表分页与目录导航逻辑
- 单目录递归删除、批量混合删除与结果聚合逻辑
- 压缩包上传会话聚合与耗时计算逻辑
- 压缩包并发上传的统计一致性与错误聚合逻辑
- CDN 刷新与资源同步编排逻辑
- CDN 目录查询接口的项目授权、参数校验与目录聚合逻辑
- 用户项目绑定快照查询与绑定编辑回填逻辑

测试层次：

- Repository 测试
  - 验证 GORM 模型、约束与查询行为
- Service 测试
  - 验证事务、权限、审计与业务编排
- Handler 测试
  - 验证请求校验、状态码与响应结构

### Frontend Testing

重点测试以下核心逻辑：

- 登录态与路由守卫
- 基于权限的菜单与按钮可见性
- 项目切换后的数据隔离
- 项目编辑场景下凭据默认保留、替换开关与参数提交一致性
- 项目配置页“CDN 域名”命名、CDN `region` 可选与 `purgeScope` 输入移除交互
- 上传、删除、重命名、刷新等关键交互流程
- 文件树层级导航、每页条数切换、目录删除与批量混合删除交互流程
- 压缩包上传会话进展、耗时与汇总信息展示
- 上传阶段 A/阶段 B 进度切换与轮询终止逻辑
- 单文件 `key` 与多文件 `keyPrefix` 参数提交与提示文案一致性
- CDN 操作台的项目/CDN/存储桶下拉联动与三类操作共享上下文行为
- CDN 页面目录查询交互与 `project_read_only`/`project_admin` 的写入口状态分离行为
- 用户绑定弹框的快照预填、紧凑表格编辑与保存后回填行为
- Overview 首页的业务核心卡片、项目工作台列表、运维风险摘要与最近活动时间线联动行为
- 语言切换后导航、按钮、提示与关键页面标题的即时更新行为
- Neo-Tech 主题在 Light/Auth/Dark 下的关键组件对比度与可读性行为
- 三套主题切换是否正确应用
- 当前目录被删除后返回上一级目录的导航行为

### Integration Boundaries

对于云厂商集成测试，建议采用分层策略：

- 单元测试中仅验证 Provider 适配器的参数转换与错误映射
- 集成测试中验证与测试环境云资源的真实交互
- 审计与权限逻辑优先在本地自动化测试覆盖

### Non-Goals for Initial Version

首版设计暂不包含以下能力：

- 移动端适配
- 自动化成本统计
- 跨项目资源复制
- 多租户企业组织层级
- 大规模异步工作流编排系统

## Traceability to Requirements

- Requirement 1: 由 Storage Component、ObjectStorageProvider、`project_buckets` 模型覆盖
- Requirement 2: 由 CDN Component、CDNProvider、异步任务状态设计覆盖
- Requirement 3: 由 Project Management Component、项目作用域中间件与项目绑定模型覆盖
- Requirement 4: 由 Authentication Component、User and RBAC Component、`user_project_roles` 模型覆盖
- Requirement 5: 由 Initialization Strategy 覆盖
- Requirement 6: 由 Audit Component、审计写入范围与可见范围控制覆盖
- Requirement 7: 由 Frontend Architecture 与 Theme Strategy 覆盖
- Requirement 8: 由 Security and Secret Management、RBAC 与错误处理规则覆盖
- Requirement 9: 由分层架构、Provider 抽象、Redis 策略与数据模型拆分覆盖
- Requirement 10: 由 User and RBAC Component 的管理员重置密码接口、审计写入与前端用户管理页面交互覆盖
- Requirement 11: 由 Storage Component 的压缩包上传会话跟踪、审计汇总聚合与前端上传结果视图覆盖
- Requirement 12: 由 Storage Component 的层级目录列表、分页控制、目录递归删除与批量混合删除覆盖
- Requirement 13: 由 Storage Component 的并发上传编排、两段进度策略与上传参数语义约束覆盖
- Requirement 14: 由 Storage Component 的文件重命名流程与目录前缀迁移重命名流程覆盖
- Requirement 15: 由 Project Management Component 的多云绑定策略、Storage/CDN Component 的按绑定项 provider 路由与阿里云 Provider 接入覆盖
- Requirement 16: 由 Project Management Component 的绑定项凭据策略、错误处理规则与前端项目编辑交互覆盖
- Requirement 17: 由 Project Management Component 的 CDN 绑定字段简化策略、CDN 刷新接口语义与前端表单命名/校验规则覆盖
- Requirement 18: 由 CDN Component 的页面交互策略、前端项目/绑定下拉联动与共享上下文操作编排覆盖
- Requirement 19: 由 Audit Component 的筛选下拉选项接口、前端平台级与项目级审计筛选联动覆盖
- Requirement 20: 由 Project Management Component 的 `accessible/context` 接口、ProjectScope 中间件与前端基于项目角色的写权限判定覆盖
- Requirement 21: 由 CDN Component 的目录查询接口、项目读写权限分离规则与前端目录查询交互覆盖
- Requirement 22: 由 User and RBAC Component 的绑定快照接口、绑定替换语义与前端紧凑绑定编辑交互覆盖
- Requirement 23: 由 Overview Workspace Component 的业务核心卡片、快捷入口、项目工作台列表、风险摘要与只读角色可见性规则覆盖
- Requirement 24: 由 Neo-Tech Theme Foundation 的主题令牌分层、三主题一致性策略与动效降级策略覆盖
- Requirement 25: 由 Internationalization Architecture 的默认语言、`en-US` 切换、持久化与缺失键回退策略覆盖
