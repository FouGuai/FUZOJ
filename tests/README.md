# 测试框架说明

## 结构

- `testutil/`: 通用测试工具包（断言、测试套件、示例）
- `auth/`: 用户认证服务相关测试（含端到端）
- `problem/`: 题目上传与清理相关测试
- `sandbox/`: 沙箱执行与 Runner/Worker 相关测试
- `judge_service/`: 判题服务相关测试
- `gateway/`: 网关相关测试
- `errors/`: 统一错误码相关测试
- `cli/`: CLI 调试客户端测试
- `test.yaml`: 端到端测试配置模板

## 运行测试

### 运行所有测试
```bash
go test ./tests/... -v
```

## 服务启动与关闭

使用测试环境时可通过脚本启动/关闭 HTTP 服务，启动后会输出各服务的 HTTP 端口号。

```bash
make -C tests start
make -C tests down
```

如需只启动/关闭某个服务：

```bash
../scripts/start_services.sh --only gateway
../scripts/stop_services.sh --only gateway
```

### 运行指定测试
```bash
go test ./tests/testutil -run TestExample -v
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

使用通用断言与测试套件时，请先引入 `fuzoj/tests/testutil`。

### 基本测试
```go
func TestMyFunction(t *testing.T) {
    got := MyFunction()
    want := "expected"
    testutil.AssertEqual(t, got, want)
}
```

### 使用测试套件
```go
func TestWithSuite(t *testing.T) {
    suite := testutil.NewTestSuite(t)
    suite.RunTest("test case", func() {
        // 测试逻辑
        testutil.AssertTrue(t, condition, "message")
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
            testutil.AssertEqual(t, got, tt.want)
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

运行前需要配置 `tests/test.yaml`：

```yaml
mysql:
  dsn: "user:password@tcp(127.0.0.1:3306)/fuzoj_test?parseTime=true&loc=Local"
redis:
  addr: "127.0.0.1:6379"
```

运行示例：

```bash
go test ./tests/auth -run TestAuthService_EndToEnd -v
```
