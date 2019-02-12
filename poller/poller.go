package poller

import (
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/akosourov/monitoring/storage"
)

// Poller периодически с интервалом Interval опрашивает список http(s) ресурсов URLs
// и замеряет доступность и время отклика, сохраняя результат в базу.
// За открытие и закрытие подключения к БД должен отвечать клиент.
type Poller struct {
	URLs     []string
	Interval time.Duration
	Timeout  time.Duration
	Workers  int
	DB       storage.Storage

	client http.Client
	stop   chan struct{}
}

// Start инициирует запуск поллера в работу. Он будет работать до тех пор,
// пока не будет вызван метод Stop или не случится ошибка записи в БД.
func (p *Poller) Start() {
	log.Println("[INFO] Start polling with interval", p.Interval)

	p.client = http.Client{
		Timeout: p.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // prevent redirect
		},
	}
	p.stop = make(chan struct{})

	var (
		wgp sync.WaitGroup // pollers
		wgs sync.WaitGroup // savers to db
	)
	in := make(chan string)
	out := make(chan *call)

	for i := 0; i < p.Workers; i++ {
		wgp.Add(1)
		go p.poll(in, out, &wgp)
		wgs.Add(1)
		go p.saveCall(out, &wgs)
	}

	tick := time.Tick(p.Interval)
	for {
		select {
		case <-tick:
			log.Println("[DEBUG] tick")

			for _, url := range p.URLs {
				in <- url
			}
		case <-p.stop:
			log.Println("[WARN] Stop poller")
			close(in)
			wgp.Wait()
			// ждем обработки записи в бд оставшихся работ
			close(out)
			wgs.Wait()
			p.stop <- struct{}{}
			log.Println("[INFO] Poller was stoped")
			return
		}
	}
}

func (p *Poller) Stop() {
	p.stop <- struct{}{} // сигнал завершения выполнения
	<-p.stop             // ожидание факта окончания работы поллера
}

type call struct {
	url         string
	isAvailable bool
	latency     int64
}

func (p *Poller) poll(in <-chan string, out chan<- *call, wg *sync.WaitGroup) {
	for url := range in {
		c, err := p.doHEAD(url)
		if err != nil {
			continue
		}
		out <- c
	}
	wg.Done()
}

func (p *Poller) saveCall(out <-chan *call, wg *sync.WaitGroup) {
	var lat int64
	for c := range out {
		lat = c.latency
		if !c.isAvailable {
			lat = -1
		}
		log.Printf("[DEBUG] SAVE: %+v", *c)
		if err := p.DB.PutLatency(c.url, lat); err != nil {
			log.Println("[ERROR] Can't PutLatency", err)
		}
	}
	wg.Done()
}

func (p *Poller) doHEAD(url string) (*call, error) {
	c := new(call)
	c.url = url
	start := time.Now()
	_, err := p.client.Head(url)
	if e, ok := err.(net.Error); ok && e.Timeout() {
		// timeout error
		return c, nil // c.isAvailable = false
	} else if err != nil {
		log.Printf("[ERROR] http error: %v", err)
		return nil, err
	}

	elapsed := time.Since(start)
	c.isAvailable = true
	c.latency = elapsed.Nanoseconds()
	return c, err
}
