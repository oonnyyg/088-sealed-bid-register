# Sealed-Bid Auction Register

本地竞价演练登记工具（纯 Go 标准库）。先收集报价承诺，再核验揭示结果，
最后计算成交名单。不连接任何支付或交易平台。

## 运行

```sh
go run ./cmd/auction --state 状态目录
```

程序从标准输入逐行读取 JSON 命令，每条命令输出一行 JSON 响应。
状态持久化在状态目录下的 `state.json`，退出后重开自动恢复。

## 命令

- `{"cmd":"create","auction":"A1","reserve":100}` 创建拍卖（保留价，单位分，非负整数）
- `{"cmd":"commit","auction":"A1","bidder":"B1","commitment":"<hex>"}` 提交承诺（仅承诺阶段）
- `{"cmd":"advance","auction":"A1"}` 切换阶段：commit → reveal → settled
- `{"cmd":"reveal","auction":"A1","bidder":"B1","amount":500,"nonce":"n1"}` 揭示报价（仅揭示阶段）
- `{"cmd":"query","auction":"A1"}` 查询拍卖状态
- `{"cmd":"settle","auction":"A1"}` 结算（仅 settled 阶段，可重复调用，结果相同）

成功响应 `{"ok":true,"result":...}`，失败响应 `{"ok":false,"error":"..."}`。
任何错误都不会改变已有记录。

## 承诺计算方法

承诺为以下字符串的 SHA-256 小写十六进制（64 个字符）：

```
拍卖号|竞买人号|报价|随机串
```

四项以竖线 `|` 连接后按 UTF-8 编码。其中报价为无前导零的非负整数十进制
（单位：分）。拍卖号、竞买人号、随机串仅允许非空 ASCII 字母或数字。

示例（Go）：

```go
sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d|%s", auction, bidder, amount, nonce)))
commitment := hex.EncodeToString(sum[:])
```

## 规则

- 拍卖编号唯一；阶段只能 commit → reveal → settled 单向推进。
- 每名竞买人每场只能提交一次承诺；阶段不符或重复提交均被拒绝。
- 揭示时重新计算承诺，匹配才记录有效报价；随机串或金额错误不消耗揭示
  机会，可重试；成功揭示后不可再改。
- 结算前 `query` 只公开承诺与是否已揭示，不输出报价或随机串；结算后
  公开所有有效报价及未揭示名单。
- 结算只考虑有效揭示：最高报价达到保留价才成交，同价取竞买人号字典序
  最小者。成交价为保留价与其余有效报价最高值的较大者；无其他有效报价
  时取保留价；无合格买家则流拍（`sold:false`）。

## 测试

```sh
go test ./...
```

覆盖：同价结算（字典序决胜、次高价成交）、失败揭示后重试、重复结算
幂等、结算前后查询可见性、状态重开恢复。
