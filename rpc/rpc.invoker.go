package rpc

import (
	"fmt"
	"strings"
	"time"

	"github.com/sereiner/parrot/rpc/balancer"
	"github.com/sereiner/lib/concurrent/cmap"
	logger "github.com/sereiner/log"
)

//Invoker RPC服务调用器，封装基于域及负载算法的RPC客户端
type Invoker struct {
	cache   cmap.ConcurrentMap
	address string
	opts    []ClientOption
	domain  string
	server  string
	lb      balancer.CustomerBalancer
	*invokerOption
}

type invokerOption struct {
	logger   *logger.Logger
	timerout time.Duration
	// balancerType int
	balancers map[string]BalancerMode
	servers   string
	// localPrefix  string
	tls map[string][]string
}

type BalancerMode struct {
	Mode  int
	Param string
}

const (
	//RoundRobin 轮询负载算法
	RoundRobin = iota + 1
	//LocalFirst 本地优先负载算法
	LocalFirst
)

//InvokerOption 客户端配置选项
type InvokerOption func(*invokerOption)

//WithInvokerLogger 设置日志记录器
func WithInvokerLogger(log *logger.Logger) InvokerOption {
	return func(o *invokerOption) {
		o.logger = log
	}
}

func WithBalancerMode(platName string, mode int, p string) InvokerOption {

	switch mode {
	case RoundRobin:
		return WithRoundRobin(platName)
	case LocalFirst:
		return WithLocalFirst(p, platName)
	default:
		return func(o *invokerOption) {
		}
	}
}

//WithRoundRobin 设置为轮询负载
func WithRoundRobin(platName ...string) InvokerOption {
	return func(o *invokerOption) {
		if len(platName) == 0 {
			o.balancers["*"] = BalancerMode{Mode: RoundRobin}
			return
		}
		for _, v := range platName {
			o.balancers[v] = BalancerMode{Mode: RoundRobin}
		}

	}
}

//WithLocalFirst 设置为本地优先负载
func WithLocalFirst(prefix string, platName ...string) InvokerOption {
	return func(o *invokerOption) {
		if prefix != "" {
			if len(platName) == 0 {
				o.balancers["*"] = BalancerMode{Mode: LocalFirst, Param: prefix}
				return
			}
			for _, v := range platName {
				o.balancers[v] = BalancerMode{Mode: LocalFirst, Param: prefix}
			}
		}
	}
}

//WithRPCTLS 设置TLS证书(pem,key)
func WithRPCTLS(platName string, tls []string) InvokerOption {
	return func(o *invokerOption) {
		if len(tls) == 2 {
			o.tls[platName] = tls
		}
	}
}

//NewInvoker 构建RPC服务调用器
//domain: 当前服务所在域
//server: 当前服务器名称
//addrss: 注册中心地址格式: zk://192.168.0.1166:2181或standalone://localhost
func NewInvoker(domain string, server string, address string, opts ...InvokerOption) (f *Invoker) {
	f = &Invoker{
		domain:  domain,
		server:  server,
		address: address,
		cache:   cmap.New(8),
		invokerOption: &invokerOption{
			// balancerType: RoundRobin,
			balancers: map[string]BalancerMode{
				"*": BalancerMode{Mode: RoundRobin},
			},
			tls: make(map[string][]string),
		},
	}
	for _, opt := range opts {
		opt(f.invokerOption)
	}
	if f.invokerOption.logger == nil {
		f.invokerOption.logger = logger.GetSession("rpc.invoker", logger.CreateSession())
	}
	return
}

//RequestFailRetry 失败重试请求
func (r *Invoker) RequestFailRetry(service string, method string, header map[string]string, form map[string]interface{}, times int) (status int, result string, params map[string]string, err error) {
	for i := 0; i < times; i++ {
		status, result, params, err = r.Request(service, method, header, form, true)
		if err == nil || status < 500 {
			return
		}
	}
	return
}

//Request 使用RPC调用Request函数
func (r *Invoker) Request(service string, method string, header map[string]string, form map[string]interface{}, failFast bool) (status int, result string, params map[string]string, err error) {
	status = 500
	client, err := r.GetClient(service)
	if err != nil {
		return
	}
	rservice, _, _, _ := ResolvePath(service, r.domain, r.server)
	status, result, params, err = client.Request(rservice, method, header, form, failFast)
	if status != 200 || err != nil {
		if err != nil {
			err = fmt.Errorf("%s请求失败:%v(%d)", service, err, status)
		} else {
			err = fmt.Errorf("%s请求失败:%d)", service, status)
		}
	}
	return
}
func (r *Invoker) getBalancer(domain string) (int, string) {
	if b, ok := r.balancers[domain]; ok {
		return b.Mode, b.Param
	}
	if b, ok := r.balancers["*"]; ok {
		return b.Mode, b.Param
	}
	return RoundRobin, ""
}

//GetClient 获取RPC客户端
//addr 支持格式:
//order.request#merchant.parrot
//order.request,order.request@api.parrot
//order.request@api
func (r *Invoker) GetClient(addr string) (c *Client, err error) {
	service, domain, server, err := ResolvePath(addr, r.domain, r.server)
	if err != nil {
		return
	}
	fullService := fmt.Sprintf(serviceRoot, strings.TrimPrefix(domain, "/"), server, service)
	_, client, err := r.cache.SetIfAbsentCb(fullService, func(i ...interface{}) (interface{}, error) {
		rsrvs := i[0].(string)
		opts := make([]ClientOption, 0, 0)
		opts = append(opts, WithLogger(r.logger))
		mode, p := r.getBalancer(domain)
		rs := balancer.NewResolver(rsrvs, time.Second, p)

		//设置负载均衡算法
		switch mode {
		case RoundRobin:
			opts = append(opts, WithRoundRobinBalancer(rs, rsrvs, time.Second, map[string]int{}))
		case LocalFirst:
			opts = append(opts, WithLocalFirstBalancer(rs, rsrvs, p, map[string]int{}))
		default:
		}

		//设置安全证书
		switch len(r.tls[domain]) {
		case 2:
			opts = append(opts, WithTLS(r.tls[domain]))
		}
		return NewClient(r.address, opts...)
	}, fullService)
	if err != nil {
		return
	}
	c = client.(*Client)
	return
}

//PreInit 预初始化服务器连接
func (r *Invoker) PreInit(services ...string) (err error) {
	for _, v := range services {
		_, err = r.GetClient(v)
		if err != nil {
			return
		}
	}
	return
}

//Close 关闭当前客户端与服务器的连接
func (r *Invoker) Close() {
	r.cache.RemoveIterCb(func(k string, v interface{}) bool {
		client := v.(*Client)
		client.Close()
		return true
	})
}

//ResolvePath   解析注册中心地址
//domain:parrot,server:merchant_cron
//order.request#merchant_api.parrot 解析为:service: /order/request,server:merchant_api,domain:parrot
//order.request 解析为 service: /order/request,server:merchant_cron,domain:parrot
//order.request#merchant_rpc 解析为 service: /order/request,server:merchant_rpc,domain:parrot
func ResolvePath(address string, d string, s string) (service string, domain string, server string, err error) {
	raddress := strings.TrimRight(address, "@")
	addrs := strings.SplitN(raddress, "@", 2)
	if len(addrs) == 1 {
		if addrs[0] == "" {
			return "", "", "", fmt.Errorf("服务地址%s不能为空", address)
		}
		service = "/" + strings.Trim(strings.Replace(raddress, ".", "/", -1), "/")
		domain = d
		server = s
		return
	}
	if addrs[0] == "" {
		return "", "", "", fmt.Errorf("%s错误，服务名不能为空", address)
	}
	if addrs[1] == "" {
		return "", "", "", fmt.Errorf("%s错误，服务名，域不能为空", address)
	}
	service = "/" + strings.Trim(strings.Replace(addrs[0], ".", "/", -1), "/")
	raddr := strings.Split(strings.TrimRight(addrs[1], "."), ".")
	if len(raddr) >= 2 && raddr[0] != "" && raddr[1] != "" {
		domain = raddr[len(raddr)-1]
		server = strings.Join(raddr[0:len(raddr)-1], ".")
		return
	}
	if len(raddr) == 1 {
		if raddr[0] == "" {
			return "", "", "", fmt.Errorf("%s错误，服务器名称不能为空", address)
		}
		domain = d
		server = raddr[0]
		return
	}
	if raddr[0] == "" && raddr[1] == "" {
		return "", "", "", fmt.Errorf(`%s错误,未指定服务器名称和域名称`, addrs[1])
	}
	domain = raddr[1]
	server = s
	return
}
