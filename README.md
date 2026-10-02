# credit — 评论者信用追溯核算库

供上层治理模块直接调用的 Go 库，数据只保存在进程内存，无任何外部依赖。

## 能力

- **事实提交** `Submit(Fact) bool`：上层提交已核实的行为事实。同一编号只有最高修订号生效；重复或较旧修订被忽略（返回 `false`）。最新修订为撤销（`Retracted: true`）时该事实不参与计分且不回退旧版本，更高修订可恢复。
- **状态查询**：
  - `Current(alias) Status`：使用全部有效事实的当前状态。
  - `At(alias, t) Status`：使用发生时刻不晚于 `t` 的有效事实（同刻全部处理），反映**当前已知修订下**的历史，而非当时系统曾显示的结果。

## 计分规则

- 每个别名独立计分，初始 50 分，每处理一条立即钳制在 [0, 100]，超出部分不结转。
- 正常评论 +5，敏感内容 -15，爽约 -20，确认违规 -40。
- 有效事实按发生时刻升序处理，同一时刻按编号升序。
- 分数 < 20 处于黑名单，达到 20 即解除；`Status.TriggerID`/`TriggerAt` 给出当前连续黑名单区间开始时、使分数从 ≥20 降到 <20 的那条触发事实。
- 补录、改时刻、改类别、撤销都会在提交返回后立即反映到所有查询（查询时全量重放核算）。

## 用法

```go
l := credit.NewLedger()

l.Submit(credit.Fact{
    ID: "f-001", Alias: "alice",
    OccurAt: time.Now(), Category: credit.CategoryNormalComment,
    Revision: 1,
})
// 修正：更高修订号替换类别
l.Submit(credit.Fact{ID: "f-001", Alias: "alice", OccurAt: t0,
    Category: credit.CategoryViolation, Revision: 2})
// 撤销
l.Submit(credit.Fact{ID: "f-001", Alias: "alice", OccurAt: t0,
    Category: credit.CategoryViolation, Revision: 3, Retracted: true})

st := l.Current("alice")            // 当前分数、黑名单状态、触发事实
hist := l.At("alice", yesterday)    // 任意历史时刻的状态
```

## 注意

- `Ledger` 非并发安全，上层并发使用时需自行加锁。
- 查询按事实数量 O(n log n) 重放，适合治理场景的调用频率。

## 测试

```sh
go test ./...
```

覆盖：上限钳制与撤销早期事实、多次进出黑名单后的补录/同刻改序/撤销触发事实/旧修订重复送达、撤销不回退旧版本、历史查询反映当前修订等场景。
