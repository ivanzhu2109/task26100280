package credit

import (
	"sort"
	"time"
)

// Ledger 信用台账，保存全部事实修订并负责核算。
// 零值不可使用，请通过 NewLedger 创建。
// Ledger 不是并发安全的，上层如需并发访问请自行加锁。
type Ledger struct {
	facts map[string]Fact // 编号 -> 当前生效的最高修订
}

// NewLedger 创建空台账。
func NewLedger() *Ledger {
	return &Ledger{facts: make(map[string]Fact)}
}

// Submit 提交一条行为事实。返回是否改变了台账状态。
// 同一编号只有最高修订号生效；重复或较旧修订不改变状态。
// 最新修订为撤销时该事实不参与计分，更高修订可以恢复它。
func (l *Ledger) Submit(f Fact) bool {
	old, ok := l.facts[f.ID]
	if ok && old.Revision >= f.Revision {
		return false
	}
	l.facts[f.ID] = f
	return true
}

// Current 查询别名当前（使用全部有效事实）的信用状态。
func (l *Ledger) Current(alias string) Status {
	return l.replay(alias, time.Time{}, false)
}

// At 查询别名在指定时刻的信用状态。
// 使用发生时刻不晚于 t 的有效事实，同刻事实全部处理后返回。
// 反映当前已知修订下的历史，而非当时系统曾经显示的结果。
func (l *Ledger) At(alias string, t time.Time) Status {
	return l.replay(alias, t, true)
}

// replay 按发生时刻升序（同刻按编号升序）重放有效事实，核算状态。
// limited 为 true 时只处理发生时刻不晚于 t 的事实。
func (l *Ledger) replay(alias string, t time.Time, limited bool) Status {
	effective := make([]Fact, 0)
	for _, f := range l.facts {
		if f.Alias != alias || f.Retracted {
			continue
		}
		if limited && f.OccurAt.After(t) {
			continue
		}
		effective = append(effective, f)
	}
	sort.Slice(effective, func(i, j int) bool {
		a, b := effective[i], effective[j]
		if !a.OccurAt.Equal(b.OccurAt) {
			return a.OccurAt.Before(b.OccurAt)
		}
		return a.ID < b.ID
	})

	st := Status{Alias: alias, Score: initialScore}
	for _, f := range effective {
		prev := st.Score
		st.Score = prev + delta(f.Category)
		if st.Score > maxScore {
			st.Score = maxScore
		}
		if st.Score < minScore {
			st.Score = minScore
		}
		// 分数从至少 20 降到不足 20 时进入新的连续黑名单区间，
		// 该事实即此次区间的触发事实。
		if prev >= blacklistThreshold && st.Score < blacklistThreshold {
			st.TriggerID = f.ID
			st.TriggerAt = f.OccurAt
		}
	}
	st.Blacklisted = st.Score < blacklistThreshold
	if !st.Blacklisted {
		st.TriggerID = ""
		st.TriggerAt = time.Time{}
	}
	return st
}
