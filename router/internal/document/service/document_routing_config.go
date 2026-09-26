package service

type DocumentRoutingConfig struct {
	// MaxInflight — Максимум одновременно исполняемых команд на процесс; лишние сразу получают HTTP 429.
	MaxInflight int `yaml:"max_inflight"`
}
