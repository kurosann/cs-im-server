package buildinfo

var current Info

func Current() Info {
	return current
}

func Set(info Info) {
	current = info
}

type Info struct {
	Version   string `json:"version"`
	Branch    string `json:"branch"`
	Revision  string `json:"revision"`
	BuildTime string `json:"buildTime"`
}

type VersionConfig struct {
	Version   string
	Branch    string
	Revision  string
	BuildTime string
}
