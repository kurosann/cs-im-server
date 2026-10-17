package main

import "vane-common/cmd"

var (
	Version   string
	Branch    string
	Revision  string
	BuildTime string
)

func version() *cmd.VersionConfig {
	return &cmd.VersionConfig{
		Version:   Version,
		Branch:    Branch,
		Revision:  Revision,
		BuildTime: BuildTime,
	}
}
