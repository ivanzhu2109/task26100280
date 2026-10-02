package credit

import (
	"sort"
	"sync"
	"time"
)

// Ledger 评论者信用台账，数据仅保存在进程内存，可并发使用。
type Ledger struct {
	mu    sync.RWMutex
	facts map[string]Fact // key 为事实编号，仅保存最高修订号
}

// NewLedger 创建空台账。
func NewLedger() *Ledger {
	return &Ledger{facts: make(map[string]Fact)}
}

// Submit 提交一条行为事实。同一编号只有最高修订号生效；
// 重复或较旧修订不改变状态。返回后所有查询立即反映完整修正。
func (l *Ledger) Submit(f Fact) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if old, ok := l.facts[f.ID]; ok && old.Revision >= f.Revision {
		return
	}
	l.facts[f.ID] = f
}

// Current 查询别名当前信用状态，使用全部有效事实。
func (l *Ledger) Current(alias string) State {
	return l.replay(alias, nil)
}

// At 查询别名在指定时刻的信用状态，使用发生时刻不晚于 t 的有效事实，
// 同刻事实全部处理后返回。反映当前已知修订下的历史。
func (l *Ledger) At(alias string, t time.Time) State {
	return l.replay(alias, &t)
}

// replay 重放别名的有效事实并计算状态；cutoff 为 nil 时不限时刻。
func (l *Ledger) replay(alias string, cutoff *time.Time) State {
	l.mu.RLock()
	effective := make([]Fact, 0)
	for _, f := range l.facts {
		if f.Alias == alias && !f.Retracted && (cutoff == nil || !f.OccurredAt.After(*cutoff)) {
			effective = append(effective, f)
		}
	}
	l.mu.RUnlock()

	// 发生时刻升序，同一时刻按编号从小到大。
	sort.Slice(effective, func(i, j int) bool {
		a, b := effective[i], effective[j]
		if !a.OccurredAt.Equal(b.OccurredAt) {
			return a.OccurredAt.Before(b.OccurredAt)
		}
		return a.ID < b.ID
	})

	state := State{Score: initialScore}
	for _, f := range effective {
		state.Score += f.Category.delta()
		if state.Score > maxScore {
			state.Score = maxScore
		}
		if state.Score < minScore {
			state.Score = minScore
		}
		if state.Blacklisted {
			if state.Score >= blacklistBelow {
				state.Blacklisted = false
				state.TriggerFactID = ""
			}
		} else if state.Score < blacklistBelow {
			state.Blacklisted = true
			state.TriggerFactID = f.ID
		}
	}
	return state
}
