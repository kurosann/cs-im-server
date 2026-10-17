package main

import (
	"vane-common/cmd"
	"vane-common/dbserver"
	"vane-common/httpserver"
	"vane-common/pkg/logger"
	"vane-common/server"

	"cs-im-server/internal/buildinfo"
	"cs-im-server/internal/cfg"
	appmodule "cs-im-server/internal/module"
)

func main() {
	_ = cfg.Init("")
	modules := appmodule.Modules()
	cmd.NewServerCmd(
		server.NewServer(cfg.ServiceRoleName).
			WithConfig(cfg.GetServerConfig).
			RegModule(modules).
			RegExtendServer(
				dbserver.EnableDBServer(cfg.GetDbConfig),
				httpserver.EnableHTTPServer(cfg.GetHTTPConfig, modules, httpserver.WithRoutePrefix("")),
			),
	).WithInit(func(configPath string) error {
		if err := cfg.Init(configPath); err != nil {
			return err
		}
		info := version()
		buildinfo.Set(buildinfo.Info{
			Version:   info.Version,
			Branch:    info.Branch,
			Revision:  info.Revision,
			BuildTime: info.BuildTime,
		})
		logger.InitLogger(cfg.GetConfig().Log)
		return nil
	}).WithVersionInfo(version()).
		Start()
}
