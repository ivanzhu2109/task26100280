package credit

import (
	"testing"
	"time"
)

var base = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func at(hour int) time.Time { return base.Add(time.Duration(hour) * time.Hour) }

func fact(id string, hour int, cat Category, rev uint64, retracted bool) Fact {
	return Fact{ID: id, Alias: "bob", OccurredAt: at(hour), Category: cat, Revision: rev, Retracted: retracted}
}

func check(t *testing.T, got State, score int, blacklisted bool, trigger string) {
	t.Helper()
	want := State{Score: score, Blacklisted: blacklisted, TriggerFactID: trigger}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// 场景一：从 50 分开始连续 12 次正常评论（封顶 100），再确认违规 -40，
// 当前分数为 60；撤销任意一条早期正常评论后，当前分数仍为 60。
func TestCapThenViolation(t *testing.T) {
	l := NewLedger()
	for i := 1; i <= 12; i++ {
		l.Submit(fact("n"+string(rune('a'+i-1)), i, CategoryNormalComment, 1, false))
	}
	l.Submit(fact("v1", 13, CategoryConfirmedViolation, 1, false))
	check(t, l.Current("bob"), 60, false, "")

	// 撤销最早的一条正常评论：少加 5 分，但原本就封顶，结果不变。
	l.Submit(fact("na", 1, CategoryNormalComment, 2, true))
	check(t, l.Current("bob"), 60, false, "")
}

// 场景二：多次进出黑名单后，经过早期补录、同刻改序、撤销触发事实、
// 旧修订重复送达与更高修订恢复，当前与历史查询均符合修正后的规则。
func TestRevisionsAndBlacklist(t *testing.T) {
	l := NewLedger()

	// 初始事实：50 -40 +5 +5 -15 -20 +5 +5 +5 +5，另有一条远期事实 m1。
	l.Submit(fact("e1", 2, CategoryConfirmedViolation, 1, false))
	l.Submit(fact("e2", 3, CategoryNormalComment, 1, false))
	l.Submit(fact("e3", 4, CategoryNormalComment, 1, false))
	l.Submit(fact("e4", 5, CategorySensitiveContent, 1, false))
	l.Submit(fact("e5", 6, CategoryNoShow, 1, false))
	l.Submit(fact("e6", 7, CategoryNormalComment, 1, false))
	l.Submit(fact("e7", 8, CategoryNormalComment, 1, false))
	l.Submit(fact("e8", 9, CategoryNormalComment, 1, false))
	l.Submit(fact("e9", 10, CategoryNormalComment, 1, false))
	l.Submit(fact("m1", 20, CategoryNormalComment, 1, false))

	// 轨迹：50→10(e1 黑)→15→20(解)→5(e4 黑)→0→5→10→15→20(解)→25
	check(t, l.Current("bob"), 25, false, "")
	check(t, l.At("bob", at(2)), 10, true, "e1")
	check(t, l.At("bob", at(4)), 20, false, "") // 20 分不在黑名单
	check(t, l.At("bob", at(6)), 0, true, "e4")
	check(t, l.At("bob", at(10)), 20, false, "")

	// 早期补录 b0（t1 敏感内容 -15）：黑名单从 e1 起连续到 e9 才解除。
	l.Submit(fact("b0", 1, CategorySensitiveContent, 1, false))
	check(t, l.Current("bob"), 25, false, "")
	check(t, l.At("bob", at(4)), 10, true, "e1") // 历史被改写
	check(t, l.At("bob", at(6)), 0, true, "e1")  // 触发事实由 e5 变为 e1
	check(t, l.At("bob", at(10)), 20, false, "")

	// 同刻改序：m1 修订为 t7 敏感内容，与 e6 同刻；e6 编号小先处理。
	// t7 前分数为 0：e6 +5 → 5，m1 -15 → 0（若顺序相反则结果不同）。
	l.Submit(fact("m1", 7, CategorySensitiveContent, 2, false))
	check(t, l.At("bob", at(7)), 0, true, "e1")
	check(t, l.Current("bob"), 15, true, "e1") // 后续 +5+5+5 只到 15，落入黑名单

	// 撤销触发事实 e1：不再参与计分，不回退旧版本；触发事实变为 e5。
	l.Submit(fact("e1", 2, CategoryConfirmedViolation, 2, true))
	check(t, l.Current("bob"), 15, true, "e5")
	check(t, l.At("bob", at(4)), 45, false, "")
	check(t, l.At("bob", at(6)), 10, true, "e5")

	// 旧修订重复送达：e1 rev1、m1 rev1、e1 rev2 重复，均不改变状态。
	l.Submit(fact("e1", 2, CategoryConfirmedViolation, 1, false))
	l.Submit(fact("m1", 20, CategoryNormalComment, 1, false))
	l.Submit(fact("e1", 2, CategoryConfirmedViolation, 2, true))
	check(t, l.Current("bob"), 15, true, "e5")
	check(t, l.At("bob", at(4)), 45, false, "")

	// 更高修订恢复 e1：重新参与计分，回到撤销前轨迹。
	l.Submit(fact("e1", 2, CategoryConfirmedViolation, 3, false))
	check(t, l.Current("bob"), 15, true, "e1")
	check(t, l.At("bob", at(4)), 10, true, "e1")

	// 别名之间互不影响。
	check(t, l.Current("alice"), 50, false, "")
}
