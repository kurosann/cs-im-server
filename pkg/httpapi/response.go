package httpapi

func WriteJSON(ctx Context, status int, code string, data any) {
	_ = ctx.JSON(status, map[string]any{"code": code, "data": data})
}

func WriteError(ctx Context, status int, code, message string) {
	_ = ctx.JSON(status, map[string]any{"code": code, "message": message})
}
