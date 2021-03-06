package mqc

import (
	"fmt"
	"sync"

	"github.com/sereiner/library/mq"
	"github.com/sereiner/parrot/conf"
	"github.com/sereiner/parrot/servers/pkg/dispatcher"
)

type Processor struct {
	*dispatcher.Dispatcher
	mq.MQConsumer
	queues        []*conf.Queue
	handles       map[string]dispatcher.HandlerFunc
	isConsume     bool
	lock          sync.Mutex
	once          sync.Once
	done          bool
	addrss        string
	raw           string
	hasAddRouters bool
}

//NewProcessor 创建processor
func NewProcessor(addrss, raw string, queues []*conf.Queue) (p *Processor, err error) {
	p = &Processor{
		Dispatcher: dispatcher.New(),
		handles:    make(map[string]dispatcher.HandlerFunc),
		addrss:     addrss,
		raw:        raw,
		queues:     queues,
	}
	fmt.Println("address: ",addrss)
	if p.MQConsumer, err = mq.NewMQConsumer(addrss, mq.WithRaw(raw)); err != nil {
		return
	}
	return p, nil
}

func (s *Processor) Close() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.done = true
	s.isConsume = false
	if s.MQConsumer != nil {
		s.once.Do(func() {
			s.MQConsumer.Close()
			s.MQConsumer = nil
		})
	}
}

func (s *Processor) Consumes() (err error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.isConsume {
		return nil
	}
	if s.MQConsumer == nil {
		s.once = sync.Once{}
		s.MQConsumer, err = mq.NewMQConsumer(s.addrss, mq.WithRaw(s.raw), mq.WithQueueCount(len(s.queues)))
		if err != nil {
			return fmt.Errorf("NewMQConsumer err:%v",err)
		}
	}
	for _, queue := range s.queues {
		err = s.Consume(queue)
		if err != nil {
			return fmt.Errorf("consume err:%v",err)
		}
	}
	s.isConsume = len(s.queues) > 0
	return nil
}

//Consume 浪费指定的队列数据
func (s *Processor) Consume(r *conf.Queue) error {
	fmt.Println("queue",r.Queue)
	return s.MQConsumer.Consume(r.Queue, r.Concurrency, func(m mq.IMessage) {
		request := newMQRequest(r.Name, "GET", m.GetMessage())
		s.HandleRequest(request)
		request = nil
	})
}
