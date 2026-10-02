// Package credit 提供评论者信用追溯核算能力。
// 数据只保存在进程内存，供上层治理模块直接调用。
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
	// CategoryViolation 确认违规，-40 分。
	CategoryViolation
)

// Fact 行为事实。编号与别名不变，其他内容可由更高修订号替换。
type Fact struct {
	ID        string    // 全局唯一编号
	Alias     string    // 评论者别名
	OccurAt   time.Time // 发生时刻
	Category  Category  // 行为类别
	Revision  uint64    // 修订号，同编号只有最高修订号生效
	Retracted bool      // 撤销标记，最新修订为撤销时该事实不参与计分
}

// Status 某一时刻的信用状态。
type Status struct {
	Alias       string    // 评论者别名
	Score       int       // 信用分，范围 [0,100]
	Blacklisted bool      // 是否处于黑名单（分数 < 20）
	TriggerID   string    // 当前连续黑名单区间的触发事实编号；不在黑名单时为空
	TriggerAt   time.Time // 触发事实的发生时刻；不在黑名单时为零值
}
