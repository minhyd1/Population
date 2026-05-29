package middleware

// ContextKey là typed key để tránh collision khi dùng context.WithValue
type ContextKey string

// Context keys dùng để truyền thông tin người dùng từ middleware xuống service layer
const (
	ContextKeyUserID   ContextKey = "audit_user_id"
	ContextKeyUsername ContextKey = "audit_username"
	ContextKeyUserRole ContextKey = "audit_user_role"
)