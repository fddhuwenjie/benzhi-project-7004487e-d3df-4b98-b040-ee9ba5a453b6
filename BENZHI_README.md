# BENZHI_README

## 项目说明
- 项目：benzhi-project-7004487e-d3df-4b98-b040-ee9ba5a453b6
- 项目用途：文物脱盐保护闭环服务提供批次、样本、方案、观测、异常复核、双人批准、档案归存和审计查询的完整 HTTP JSON API，支持回环地址配置、乐观并发和幂等请求。
- Go 工具链：`golang:1.26`
- 前端工具链：无

## 项目描述
- 项目名称：文物脱盐保护闭环服务
- 项目概述：面向文物保护人员的脱盐处理闭环服务，跟踪样本登记、方案编制、处理观测、异常复核、质量批准与档案归存的状态变化。
- 核心工作流：保护批次创建→样本登记与完整性校验→分阶段脱盐方案确认→任务执行与观测记录→异常暂停复核→阶段完成申请→双人质量批准→保护档案归存
- 对外接口：HTTP JSON API 提供批次、样本、方案、观测、复核和档案接口；服务支持 -addr=127.0.0.1:<port> 或 PORT 环境变量，默认监听 127.0.0.1:19081，禁止绑定 0.0.0.0。

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/server -addr=127.0.0.1:19081 -self-check
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-7004487e-d3df-4b98-b040-ee9ba5a453b6-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-7004487e-d3df-4b98-b040-ee9ba5a453b6-arm64 linux/arm64
docker run -it benzhi-project-7004487e-d3df-4b98-b040-ee9ba5a453b6-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -addr=127.0.0.1:19081 -self-check`
