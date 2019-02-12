package cmd

import (
	"bufio"
	"os"
	"strings"
)

func MakeURLs(fname string) ([]string, error) {
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
