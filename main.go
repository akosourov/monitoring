package main

import (
	"bufio"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/akosourov/monitoring/storage"
)

func main() {
	sitesfname := flag.String("sites", "sites.txt", "Filename with list of sites")
	interval := flag.String("interval", "10s", "Fetch interval")
	flag.Parse()

	dur, err := time.ParseDuration(*interval)
	if err != nil {
		log.Fatal("[ERROR] ", err)
	}

	urls, err := makeURLs(*sitesfname)
	if err != nil {
		log.Fatal("[ERROR] ", err)
	}

	db := storage.New("./bolt.db", urls)

	// graceful shutdown
	sigs := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		log.Println("[WARN] catch signal ", sig)
		close(done)
	}()

	getStatus(urls, dur, done, db)
}

func makeURLs(fname string) ([]string, error) {
	f, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	urls := []string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		url := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			url = "https://" + url
		}
		urls = append(urls, url)
	}
	if scanner.Err() != nil {
		return nil, err
	}

	return urls, nil
}

func ping(urls []string, db storage.Storage) {
	client := http.Client{
		Timeout: 2 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // prevent redirect
		},
	}

	var wg sync.WaitGroup
	for _, url := range urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()

			log.Println("[DEBUG] HTTP HEAD ", u)

			start := time.Now()

			resp, err := client.Head(u)

			if err == http.ErrHandlerTimeout {
				log.Println("TIMEOUT")
				db.PutLatency(u, -1)
			} else if err != nil {
				log.Println("[ERROR] HTTP HEAD ", err)
			} else {
				log.Println(u, resp.Status)
				db.PutLatency(u, time.Since(start).Nanoseconds())
			}

		}(url)
	}
	wg.Wait()
}

func getStatus(sites []string, dur time.Duration, done chan struct{}, db storage.Storage) {

	tick := time.Tick(dur)
	for {
		select {
		case <-tick:
			log.Println("TICK")
			ping(sites, db)
		case <-done:
			log.Println("Stopping")
			db.Close()
			return
			// close(tasks)
		}
	}

	// producer
	go func() {
		var i int
		for {
			if i == len(sites)-1 {
				i = 0
			}
			// tasks <- sites[i]
			i++
		}
	}()

	// workers
	tasks := make(chan string)
	for i := 0; i < 4; i++ {
		go func() {
			for _ = range tasks {

			}
		}()
	}
}
