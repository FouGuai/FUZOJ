# 测试框架说明

## 结构

- `testing_suite.go`: 测试套件基础功能，提供统一的 setup 和 teardown
- `helpers.go`: 测试辅助函数，包括断言和工具函数
- `example_test.go`: 示例测试，展示如何使用测试框架
- `error_test.go`: 错误处理相关测试

## 运行测试

### 运行所有测试
```bash
go test ./tests/... -v
```

### 运行指定测试
```bash
go test ./tests -run TestExample -v
```

### 运行基准测试
```bash
go test ./tests -bench=. -benchmem
```

### 生成覆盖率报告
```bash
go test ./tests/... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

### 运行竞态检测
```bash
go test ./tests/... -race
```

## 编写测试

### 基本测试
```go
func TestMyFunction(t *testing.T) {
    got := MyFunction()
    want := "expected"
    AssertEqual(t, got, want)
}
```

### 使用测试套件
```go
func TestWithSuite(t *testing.T) {
    suite := NewTestSuite(t)
    suite.RunTest("test case", func() {
        // 测试逻辑
        AssertTrue(t, condition, "message")
    })
}
```

### 表驱动测试
```go
func TestTableDriven(t *testing.T) {
    tests := []struct {
        name string
        input int
        want int
    }{
        {"case 1", 1, 2},
        {"case 2", 2, 4},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := MyFunction(tt.input)
            AssertEqual(t, got, tt.want)
        })
    }
}
```

## CI/CD

测试会在以下情况自动运行：
- 每次 push 到任意分支
- 每次创建 Pull Request
- 可以在 GitHub Actions 页面手动触发

查看测试结果：前往 GitHub 仓库的 Actions 标签页。

## 端到端测试（Auth Service）

该测试会通过 HTTP 接口调用认证流程，并校验 MySQL 与 Redis 的落库/缓存结果。
测试会自动执行建表（`users`、`user_tokens`）。

运行前需要配置仓库根目录的 `test.yaml`：

```yaml
mysql:
  dsn: "user:password@tcp(127.0.0.1:3306)/fuzoj_test?parseTime=true&loc=Local"
redis:
  addr: "127.0.0.1:6379"
```

运行示例：

```bash
go test ./tests -run TestAuthService_EndToEnd -v
```
