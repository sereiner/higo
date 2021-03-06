package cron

import (
	logger "github.com/sereiner/library/log"
	"github.com/sereiner/parrot/conf"
	"github.com/sereiner/parrot/servers"
)

type rpcServerAdapter struct {
}

func (h *rpcServerAdapter) Resolve(c string, conf conf.IServerConf, log *logger.Logger) (servers.IRegistryServer, error) {
	return NewCronResponsiveServer(c, conf, log)
}

func init() {
	servers.Register("cron", &rpcServerAdapter{})
}
