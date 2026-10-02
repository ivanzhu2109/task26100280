package credit

// 计分规则常量。
const (
	initialScore       = 50 // 初始分数
	minScore           = 0
	maxScore           = 100
	blacklistThreshold = 20 // 低于 20 进入黑名单，达到 20 解除
)

// delta 返回行为类别的分数变化。
func delta(c Category) int {
	switch c {
	case CategoryNormalComment:
		return 5
	case CategorySensitiveContent:
		return -15
	case CategoryNoShow:
		return -20
	case CategoryViolation:
		return -40
	default:
		return 0
	}
}
