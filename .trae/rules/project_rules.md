# 项目规则

## 部署约束（用户明确要求，必须遵守）
- 本项目**不使用 Docker**：不要编写或修改 Dockerfile / docker-compose，不要建议 Docker 相关的部署或功能方案。部署一律采用传统方式（后端编译二进制直接运行 + 前端静态资源）。

## 验证命令
- 后端：`cd backend && go build ./... && go test ./internal/service/ ./internal/handler/ -count=1`
- 前端：`cd frontend &&  npx vue-tsc --noEmit`

## 本地开发环境启动命令
- 后端：`cd backend && go run ./cmd/server    ==== go build ./... && go test ./internal/service/ ./internal/handler/ -count=1`
- 前端：`cd frontend &&  npx vite`