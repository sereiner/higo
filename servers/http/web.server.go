package http

import (
	"context"
	"fmt"
	x "net/http"
	"strings"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/gin-gonic/gin"
	logger "github.com/sereiner/library/log"
	"github.com/sereiner/library/net"
	"github.com/sereiner/parrot/conf"
	"github.com/sereiner/parrot/servers"
	"github.com/sereiner/parrot/servers/http/middleware"
)

//WebServer web服务器
type WebServer struct {
	*option
	conf    *conf.MetadataConf
	engine  *x.Server
	gin     *gin.Engine
	views   []string
	running string
	proto   string
	host    string
	port    string
}

//NewWebServer 创建web服务器
func NewWebServer(name string, addr string, routers []*conf.Router, opts ...Option) (t *WebServer, err error) {
	t = &WebServer{conf: &conf.MetadataConf{
		Name: name,
		Type: "web",
	}}
	t.option = &option{
		metric:            middleware.NewMetric(t.conf),
		readHeaderTimeout: 6,
		readTimeout:       6,
		writeTimeout:      6}
	for _, opt := range opts {
		opt(t.option)
	}
	t.conf.Name = fmt.Sprintf("%s.%s.%s", t.platName, t.systemName, t.clusterName)
	if t.Logger == nil {
		t.Logger = logger.GetSession(name, logger.CreateSession())
	}
	naddr, err := t.getAddress(addr)
	if err != nil {
		return nil, err
	}
	t.engine = &x.Server{
		Addr:              naddr,
		ReadHeaderTimeout: time.Second * time.Duration(t.option.readHeaderTimeout),
		ReadTimeout:       time.Second * time.Duration(t.option.readTimeout),
		WriteTimeout:      time.Second * time.Duration(t.option.writeTimeout),
		MaxHeaderBytes:    1 << 20,
	}
	if routers != nil {
		t.engine.Handler, err = t.getHandler(routers)
	}
	t.SetTrace(t.showTrace)
	return
}

// Run the http server
func (s *WebServer) Run() error {
	s.proto = "http"
	s.running = servers.ST_RUNNING
	errChan := make(chan error, 1)
	go func(ch chan error) {
		if err := s.engine.ListenAndServe(); err != nil {
			ch <- err
		}
	}(errChan)
	select {
	case <-time.After(time.Millisecond * 500):
		return nil
	case err := <-errChan:
		s.running = servers.ST_STOP
		return err
	}
}

//RunTLS RunTLS server
func (s *WebServer) RunTLS(certFile, keyFile string) error {
	s.proto = "https"
	s.running = servers.ST_RUNNING
	errChan := make(chan error, 1)
	go func(ch chan error) {
		if err := s.engine.ListenAndServeTLS(certFile, keyFile); err != nil {
			ch <- err
		}
	}(errChan)
	select {
	case <-time.After(time.Millisecond * 500):
		return nil
	case err := <-errChan:
		s.running = servers.ST_STOP
		return err
	}
}

//Shutdown 关闭服务器
func (s *WebServer) Shutdown(timeout time.Duration) {
	if s.engine != nil {
		s.metric.Stop()
		s.running = servers.ST_STOP
		ctx, cannel := context.WithTimeout(context.Background(), timeout)
		defer cannel()
		if err := s.engine.Shutdown(ctx); err != nil {
			if err == x.ErrServerClosed {
				s.Infof("%s:已关闭", s.conf.Name)
				return
			}
			s.Errorf("%s关闭出现错误:%v", s.conf.Name, err)
		}
	}
}

//GetAddress 获取当前服务地址
func (s *WebServer) GetAddress(h ...string) string {
	if len(h) > 0 && h[0] != "" {
		return fmt.Sprintf("%s://%s:%s", s.proto, h[0], s.port)
	}
	return fmt.Sprintf("%s://%s:%s", s.proto, s.host, s.port)
}

//GetStatus 获取当前服务器状态
func (s *WebServer) GetStatus() string {
	return s.running
}

func (s *WebServer) getAddress(addr string) (string, error) {
	host := "0.0.0.0"
	port := "8081"
	args := strings.Split(addr, ":")
	l := len(args)
	if addr == "" {
		l = 0
	}
	switch l {
	case 0:
	case 1:
		if govalidator.IsPort(args[0]) {
			port = args[0]
		} else {
			host = args[0]
		}
	case 2:
		host = args[0]
		port = args[1]
	default:
		return "", fmt.Errorf("%s地址不合法", addr)
	}
	switch host {
	case "0.0.0.0", "":
		s.host = net.GetLocalIPAddress()
	case "127.0.0.1", "localhost":
		s.host = host
	default:
		if net.GetLocalIPAddress(host) != host {
			return "", fmt.Errorf("%s地址不合法", addr)
		}
		s.host = host
	}

	if !govalidator.IsPort(port) {
		return "", fmt.Errorf("%s端口不合法", addr)
	}
	if port == "80" {
		if err := checkPrivileges(); err != nil {
			return "", err
		}
	}
	s.port = port
	return fmt.Sprintf("%s:%s", host, s.port), nil
}
