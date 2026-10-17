package module

import (
	"cs-im-server/internal/module/gateway"
	"cs-im-server/internal/module/system"
	commonmodule "vane-common/module"
)

func Modules() []commonmodule.ModuleInfo {
	return []commonmodule.ModuleInfo{
		commonmodule.NewModule("system", system.NewSystemModule()),
		commonmodule.NewModule("gateway", gateway.NewGatewayModule()),
	}
}
