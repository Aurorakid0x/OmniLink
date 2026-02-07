package job

// SupportedEventKey 系统支持的事件触发器常量定义
const (
	EventKeyUserLogin      = "user_login"       // 用户登录
	EventKeyNewFriendApply = "new_friend_apply" // 收到好友申请 (待实现)
	EventKeyGroupMention   = "group_mention"    // 群内被@提及 (待实现)
)

// AllSupportedEvents 返回所有支持的事件key及其描述
func AllSupportedEvents() map[string]string {
	return map[string]string{
		EventKeyUserLogin:      "用户登录时触发",
		EventKeyNewFriendApply: "收到好友申请时触发 (Todo)",
		EventKeyGroupMention:   "群里被@时触发 (Todo)",
	}
}

// IsValidEventKey 校验事件key是否有效
func IsValidEventKey(key string) bool {
	_, ok := AllSupportedEvents()[key]
	return ok
}
