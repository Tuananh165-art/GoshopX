package contextkeys

type ctxKeyUserID struct{}

var UserIDKey = ctxKeyUserID{}

type ctxKeyRole struct{}

var RoleKey = ctxKeyRole{}

type ctxKeyClientIP struct{}

var ClientIPKey = ctxKeyClientIP{}
