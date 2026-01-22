# 贡献指南 (Contributing Guide)

感谢你对 `broker` 项目感兴趣！我们欢迎各种形式的贡献，包括修复 Bug、改进文档、提出新功能建议或实现新的适配器。

## 参与方式

1.  **提交 Issue**: 发现 Bug 或有新想法？请先[提交 Issue](https://github.com/qvcloud/broker/issues)。
2.  **Pull Request**: 想要贡献代码？请遵循以下流程：
    *   Fork 本仓库。
    *   在你的本地分支上进行修改。
    *   确保所有测试通过 (`make test`)。
    *   提交 PR 并关联相关的 Issue。

## 开发环境设置

*   **Go 版本**: 项目要求 Go 1.24+。
*   **工具**: 推荐安装 `golangci-lint`。

```bash
# 克隆仓库
git clone https://github.com/qvcloud/broker.git
cd broker

# 下载依赖
go mod download

# 运行测试
make test

# 运行 Lint
make lint
```

## 代码规范

*   遵循官方 Go 编码规范。
*   所有公开的方法、结构体都应有中文（或英文）注释。
*   保持接口的一致性，新的适配器应参考现有适配器的实现。

## 实现新的适配器

如果你想支持一个新的消息中间件：
1. 请参考 `brokers/` 目录下的现有实现（如 `redis` 或 `nats`）。
2. 在 `brokers/` 下创建新目录。
3. 实现 `broker.Broker` 接口。
4. 提供必要的单元测试。
5. 在 `ADAPTER_EXTENSIONS.md` 中记录特定的 Option。
