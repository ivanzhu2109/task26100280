package credit

import "time"

// Category 行为类别。
type Category int

const (
	// CategoryNormalComment 核实的正常评论，+5 分。
	CategoryNormalComment Category = iota
	// CategorySensitiveContent 敏感内容，-15 分。
	CategorySensitiveContent
	// CategoryNoShow 爽约，-20 分。
	CategoryNoShow
	// CategoryConfirmedViolation 确认违规，-40 分。
	CategoryConfirmedViolation
)

// delta 返回该类别对信用分的影响。
func (c Category) delta() int {
	switch c {
	case CategoryNormalComment:
		return 5
	case CategorySensitiveContent:
		return -15
	case CategoryNoShow:
		return -20
	case CategoryConfirmedViolation:
		return -40
	}
	return 0
}

// Fact 行为事实。ID 与 Alias 不可变，其余字段可由更高 Revision 替换。
type Fact struct {
	ID         string    // 全局唯一编号
	Alias      string    // 评论者别名
	OccurredAt time.Time // 发生时刻
	Category   Category  // 行为类别
	Revision   uint64    // 修订号，同编号只有最高修订号生效
	Retracted  bool      // 撤销标记，为 true 时该事实不参与计分
}

// State 某一时刻的信用状态。
type State struct {
	Score         int    // 信用分，范围 [0,100]
	Blacklisted   bool   // 是否处于黑名单（分数 < 20）
	TriggerFactID string // 当前连续黑名单区间的触发事实编号；不在黑名单时为空
}

const (
	initialScore   = 50
	minScore       = 0
	maxScore       = 100
	blacklistBelow = 20 // 分数低于该值进入黑名单，达到该值解除
)
