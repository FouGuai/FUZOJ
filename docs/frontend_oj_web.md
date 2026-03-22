# OJ Frontend

## 功能概览

`web/` 提供一套独立的 Vue 3 前端，用于承接登录、题库浏览、在线判题和题目发布四类核心场景。界面风格参考 LeetCode，采用白色主色和双栏做题布局：左侧阅读题面，右侧写代码并实时查看判题结果。系统通过网关 HTTP 接口完成认证、题面拉取、提交判题和题目创建，通过 Status SSE 接口接收提交状态推送，SSE 失败时自动降级为轮询，确保本地开发和异常网络下仍可用。

## 关键接口或数据结构

- `src/api/http.ts`：统一 Axios 客户端，负责 Token 注入、401 自动刷新和错误归一化。
- `src/stores/auth.ts`：会话状态管理，持久化 access/refresh token 与用户信息。
- `src/pages/ProblemWorkspacePage.vue`：做题主页面，集成题面展示、Monaco 编辑器、代码草稿缓存与提交状态面板。
- `src/pages/ProblemCreatePage.vue`：出题工作台，串联创建题目、分片上传、题面保存和发布版本。
- `src/composables/useSubmissionStream.ts`：封装 SSE 订阅与轮询降级逻辑。

## 使用示例或配置说明

前端工程位于 `web/`，使用 Vite 启动。配置文件采用 `.env` 方式指定网关地址，例如：

- `VITE_API_BASE_URL=http://127.0.0.1:8080`
- `VITE_SSE_BASE_URL=http://127.0.0.1:8080`

常用流程：
1. 进入 `/login` 登录或注册。
2. 在 `/problems` 浏览公开题库。
3. 进入 `/problems/:id` 做题并提交，页面自动订阅判题状态。
4. 登录后进入 `/problemset/new` 创建题目、上传数据包、编辑题面并发布。
