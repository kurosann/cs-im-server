package service

import (
	appmodel "cs-im-server/internal/module/gateway/internal/app/model"
	realtimepkg "cs-im-server/internal/module/gateway/internal/realtime/pkg"
	iampkg "cs-im-server/internal/module/iam/pkg"
)

type App = appmodel.App
type Principal = iampkg.Principal
type RealtimeService = realtimepkg.RealtimeService
