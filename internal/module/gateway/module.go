package gateway

import (
	kernel "cs-im-server/internal/module/gateway/internal/kernel"

	commonmodule "vane-common/module"
)

type Module = kernel.Module

var _ commonmodule.IModule = (*Module)(nil)
