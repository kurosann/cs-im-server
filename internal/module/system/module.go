package system

import (
	"context"

	"cs-im-server/internal/buildinfo"
	commonmodule "vane-common/module"
)

var _ commonmodule.IModule = (*SystemModule)(nil)
var _ commonmodule.IHTTPModule = (*SystemModule)(nil)

type SystemModule struct{}

func NewSystemModule() *SystemModule {
	return &SystemModule{}
}

func (m *SystemModule) Init(_ context.Context) error {
	return nil
}

func (m *SystemModule) Api() []commonmodule.ApiRegInfo {
	return []commonmodule.ApiRegInfo{}
}

func (m *SystemModule) Job() []commonmodule.JobRegInfo {
	return []commonmodule.JobRegInfo{}
}

func (m *SystemModule) Task() []commonmodule.TaskRegInfo {
	return []commonmodule.TaskRegInfo{}
}

func (m *SystemModule) HttpApi() commonmodule.HttpApiInfo {
	return commonmodule.HttpApiInfo{
		Prefix: "",
		Handlers: []commonmodule.HttpHandlerRegInfo{
			commonmodule.NewGetReqHttpRegInfo("/api/v1/version", m.version),
		},
	}
}

func (m *SystemModule) version(ctx commonmodule.HttpReqContext) {
	ctx.JSON(200, map[string]any{"code": "OK", "data": buildinfo.Current()})
}
