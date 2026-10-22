package module

import (
	"cs-im-server/internal/module/ai"
	"cs-im-server/internal/module/channel"
	"cs-im-server/internal/module/gateway"
	"cs-im-server/internal/module/iam"
	"cs-im-server/internal/module/llm"
	"cs-im-server/internal/module/relay"
	"cs-im-server/internal/module/system"
	"cs-im-server/internal/module/translation"
	commonmodule "vane-common/module"
)

func Modules() []commonmodule.ModuleInfo {
	return []commonmodule.ModuleInfo{
		commonmodule.NewModule("system", system.NewSystemModule()),
		commonmodule.NewModule("iam", &iam.Module{}),
		commonmodule.NewModule("ai", &ai.Module{}),
		commonmodule.NewModule("gateway", &gateway.Module{}),
		commonmodule.NewModule("llm", llm.NewModule("")),
		commonmodule.NewModule("relay", &relay.Module{}),
		commonmodule.NewModule("channel", &channel.Module{}),
		commonmodule.NewModule("translation", &translation.Module{}),
	}
}
