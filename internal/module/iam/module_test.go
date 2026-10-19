package iam

import (
	"context"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestIAMHttpApiRoutesRegister(t *testing.T) {
	gin.SetMode(gin.TestMode)
	module := &Module{}
	if err := module.Init(context.Background()); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	engine := gin.New()
	for _, route := range module.HttpApi().Handlers {
		if route.Handler == nil {
			continue
		}
		engine.Handle(route.Method, route.Path, func(*gin.Context) {})
	}
}
