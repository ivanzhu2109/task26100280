# credit — 评论者信用追溯核算库

供上层治理模块直接调用的 Go 库，数据仅保存在进程内存，无任何外部依赖。
上层负责核实事实，本库负责按最新修订重放事实并核算信用状态。

## 核心概念

- **事实（Fact）**：包含全局唯一编号、评论者别名、发生时刻、行为类别、修订号、撤销标记。
  编号与别名不可变，其余字段由更高修订号整体替换。同一编号只有最高修订号生效；
  重复或较旧修订被忽略。最新修订为撤销时该事实不参与计分（不回退旧版本），
  更高修订可恢复它。
- **计分**：每个别名独立计分，初始 50 分。正常评论 +5，敏感内容 -15，
  爽约 -20，确认违规 -40。有效事实按发生时刻升序、同刻按编号升序逐条处理，
  每处理一条立即将分数截断到 [0, 100]，超出部分不结转。
- **黑名单**：分数低于 20 进入，达到 20 解除。触发事实是本次连续黑名单区间
  开始时使分数从至少 20 降到不足 20 的那条事实；不在黑名单时触发事实为空。
- **查询**：`Current` 使用全部有效事实；`At` 使用发生时刻不晚于指定时刻的
  有效事实（同刻全部处理后返回）。历史查询反映当前已知修订下的历史，
  而非当时系统曾显示的结果。`Submit` 返回后所有查询立即反映完整修正。

## 使用示例

```go
l := credit.NewLedger()

l.Submit(credit.Fact{
    ID: "f-001", Alias: "bob",
    OccurredAt: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
    Category: credit.CategoryConfirmedViolation, Revision: 1,
})

// 申诉成功：以更高修订号撤销
l.Submit(credit.Fact{
    ID: "f-001", Alias: "bob",
    OccurredAt: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
    Category: credit.CategoryConfirmedViolation, Revision: 2, Retracted: true,
})

now := l.Current("bob")        // 当前状态
hist := l.At("bob", someTime)  // 历史时刻状态
// State{Score, Blacklisted, TriggerFactID}
```

## 行为类别与分值

| 类别 | 常量 | 分值 |
| --- | --- | --- |
| 正常评论 | `CategoryNormalComment` | +5 |
| 敏感内容 | `CategorySensitiveContent` | -15 |
| 爽约 | `CategoryNoShow` | -20 |
| 确认违规 | `CategoryConfirmedViolation` | -40 |

## 测试

```sh
go test ./...
```

覆盖：封顶后违规再撤销早期评论（60 分不变）；多次进出黑名单后经早期补录、
同刻改序、撤销触发事实、旧修订重复送达、更高修订恢复后的当前与历史查询；
20 分不在黑名单；别名单独立计分。

## 并发与复杂度

`Ledger` 方法均可并发调用。每次查询按当前事实集全量重放，提交为 O(1)，
查询为 O(n log n)（n 为该别名的有效事实数）。数据随进程退出而消失，
如需持久化，上层可自行保存事实流并在重启后重放提交。
