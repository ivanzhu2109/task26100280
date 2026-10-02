package credit

import (
	"testing"
	"time"
)

var base = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func at(min int) time.Time { return base.Add(time.Duration(min) * time.Minute) }

func normal(id, alias string, min int, rev uint64) Fact {
	return Fact{ID: id, Alias: alias, OccurAt: at(min), Category: CategoryNormalComment, Revision: rev}
}

func check(t *testing.T, st Status, score int, blacklisted bool, trigger string) {
	t.Helper()
	if st.Score != score {
		t.Errorf("score = %d, want %d", st.Score, score)
	}
	if st.Blacklisted != blacklisted {
		t.Errorf("blacklisted = %v, want %v (score %d)", st.Blacklisted, blacklisted, st.Score)
	}
	if st.TriggerID != trigger {
		t.Errorf("trigger = %q, want %q", st.TriggerID, trigger)
	}
	if !blacklisted && !st.TriggerAt.IsZero() {
		t.Errorf("TriggerAt should be zero when not blacklisted, got %v", st.TriggerAt)
	}
}

// 场景一：从 50 分连续 12 次正常评论，再一次确认违规，当前 60 分；
// 撤销任意一条早期正常评论后，当前仍为 60 分（上限截断不结转）。
func TestClampAndRetract(t *testing.T) {
	l := NewLedger()
	const alias = "alice"
	for i := 0; i < 12; i++ {
		l.Submit(normal("n"+string(rune('a'+i)), alias, i, 1))
	}
	l.Submit(Fact{ID: "v1", Alias: alias, OccurAt: at(12), Category: CategoryViolation, Revision: 1})
	check(t, l.Current(alias), 60, false, "")

	// 撤销最早的一条正常评论（更高修订 + 撤销标记）。
	l.Submit(Fact{ID: "na", Alias: alias, OccurAt: at(0), Category: CategoryNormalComment, Revision: 2, Retracted: true})
	check(t, l.Current(alias), 60, false, "")
	// 历史查询同样反映修正：第 12 条事实处理前分数被钳制在 100。
	check(t, l.At(alias, at(11)), 100, false, "")
}

// 场景二：多次进出黑名单，随后补录、同刻改序、撤销触发事实、旧修订重复送达。
func TestCorrections(t *testing.T) {
	l := NewLedger()
	const alias = "bob"

	// 初始事实序列。
	l.Submit(Fact{ID: "v1", Alias: alias, OccurAt: at(0), Category: CategoryViolation, Revision: 1})        // 50->10 进黑
	l.Submit(normal("n1", alias, 1, 1))                                                                     // 10->15
	l.Submit(normal("n2", alias, 2, 1))                                                                     // 15->20 出黑
	l.Submit(Fact{ID: "s1", Alias: alias, OccurAt: at(3), Category: CategorySensitiveContent, Revision: 1}) // 20->5 进黑
	l.Submit(normal("n3", alias, 4, 1))                                                                     // 5->10
	l.Submit(normal("n4", alias, 5, 1))                                                                     // 10->15
	l.Submit(normal("n5", alias, 6, 1))                                                                     // 15->20 出黑

	check(t, l.Current(alias), 20, false, "")
	check(t, l.At(alias, at(0)), 10, true, "v1")
	check(t, l.At(alias, at(2)), 20, false, "") // 20 分不在黑名单
	check(t, l.At(alias, at(4)), 10, true, "s1")

	// 早期补录：在最早事实之前补一条正常评论，改变后续整条轨迹。
	l.Submit(normal("n0", alias, -1, 1))
	// 55,15(v1进黑),20(n1出黑),25,10(s1进黑),15,20(n4出黑),25
	check(t, l.Current(alias), 25, false, "")
	check(t, l.At(alias, at(0)), 15, true, "v1")
	check(t, l.At(alias, at(1)), 20, false, "")
	check(t, l.At(alias, at(3)), 10, true, "s1")
	check(t, l.At(alias, at(5)), 20, false, "")

	// 同刻事实改序：n2 改到与 s1 同一时刻，按编号升序 n2 先处理。
	l.Submit(normal("n2", alias, 3, 2))
	// 55,15(v1进黑),20(n1出黑),5(s1进黑),10(n2),15,20(n4出黑),25
	check(t, l.Current(alias), 25, false, "")
	check(t, l.At(alias, at(3)), 10, true, "s1") // 同刻全部处理后返回
	check(t, l.At(alias, at(4)), 15, true, "s1")
	check(t, l.At(alias, at(5)), 20, false, "")

	// 撤销触发事实 s1：第二段黑名单区间整体消失。
	l.Submit(Fact{ID: "s1", Alias: alias, OccurAt: at(3), Category: CategorySensitiveContent, Revision: 2, Retracted: true})
	// 55,15(v1进黑),20(n1出黑),25,30,35,40
	check(t, l.Current(alias), 40, false, "")
	check(t, l.At(alias, at(0)), 15, true, "v1")
	check(t, l.At(alias, at(3)), 25, false, "")

	// 旧修订重复送达：s1 的修订 1（未撤销）与 v1 的修订 1 重发，均不改变状态。
	if l.Submit(Fact{ID: "s1", Alias: alias, OccurAt: at(3), Category: CategorySensitiveContent, Revision: 1}) {
		t.Error("older revision should not change state")
	}
	if l.Submit(Fact{ID: "v1", Alias: alias, OccurAt: at(0), Category: CategoryViolation, Revision: 1}) {
		t.Error("duplicate revision should not change state")
	}
	check(t, l.Current(alias), 40, false, "")
	check(t, l.At(alias, at(0)), 15, true, "v1")

	// 更高修订恢复被撤销的事实：s1 修订 3 恢复计分。
	l.Submit(Fact{ID: "s1", Alias: alias, OccurAt: at(3), Category: CategorySensitiveContent, Revision: 3})
	// 55,15(v1进黑),20(n1出黑),25(n2),10(s1进黑),15,20(n4出黑),25
	check(t, l.Current(alias), 25, false, "")
	check(t, l.At(alias, at(3)), 10, true, "s1")
	check(t, l.At(alias, at(4)), 15, true, "s1")
}

// 撤销后不回退到旧版本：撤销是终态，除非更高修订恢复。
func TestRetractDoesNotFallBack(t *testing.T) {
	l := NewLedger()
	const alias = "carol"
	l.Submit(Fact{ID: "x", Alias: alias, OccurAt: at(0), Category: CategoryViolation, Revision: 1})
	l.Submit(Fact{ID: "x", Alias: alias, OccurAt: at(0), Category: CategoryViolation, Revision: 2, Retracted: true})
	// 撤销生效，不回退到修订 1 的 -40。
	check(t, l.Current(alias), 50, false, "")
}

// 历史查询反映当前已知修订下的历史，而非当时显示的结果。
func TestHistoryReflectsCurrentRevisions(t *testing.T) {
	l := NewLedger()
	const alias = "dave"
	l.Submit(normal("n1", alias, 0, 1))
	check(t, l.At(alias, at(0)), 55, false, "")
	// 事后把该事实改为确认违规，历史查询同一时刻结果随之改变。
	l.Submit(Fact{ID: "n1", Alias: alias, OccurAt: at(0), Category: CategoryViolation, Revision: 2})
	check(t, l.At(alias, at(0)), 10, true, "n1")
}
