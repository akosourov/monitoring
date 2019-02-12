// Клиент запуска поллера. Необходим для отладки
package main

import (
	"bufio"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/akosourov/monitoring/poller"
	"github.com/akosourov/monitoring/storage"
)

func main() {
	sitesfname := flag.String("sites", "sites.txt", "Filename with list of sites")
	interval := flag.String("interval", "10s", "Poll interval")
	flag.Parse()

	dur, err := time.ParseDuration(*interval)
	if err != nil {
		log.Fatal("[ERROR] ", err)
	}

	urls, err := makeURLs(*sitesfname)
	if err != nil {
		log.Fatal("[ERROR] ", err)
	}

	db := storage.New("./bolt.db")
	p := poller.Poller{
		URLs:     urls,
		Interval: dur,
		Timeout:  2 * time.Second,
		DB:       db,
		Workers:  8,
	}

	// graceful shutdown
	sigs := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		log.Println("[WARN] catch signal ", sig)
		close(done)
		p.Stop()
		db.Close()
	}()

	p.Start()
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
