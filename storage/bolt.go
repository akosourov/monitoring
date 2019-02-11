/*
Реализация хранилища с помощью boltdb.

Схема данных:
Два внешних бакета:
	1. "Latency" - содержит вложенные бакеты, каждый вложенный бакет - это url сервиса,
		в котором ключ - timestamp (нс), а значение - время отклика (нс). В случае, если
		сервис недоступен пишется -1

	2. "AvgLatency" - среднее значения время отклика (нс) сервиса
		ключ - url сервиса,
		значение - json вида {Count: int, Sum: int64, Avg: int64},
			где Count - общее число записей,
				Sum - сумма времени отклика,
				Avg - среднеарифметическое время отклика (нс)
*/

package storage

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	latencyBucketName    = "Latency"
	avgLatencyBucketName = "AvgLatency"
)

type boltStorage struct {
	path string
	db   *bolt.DB
}

func NewBoltStorage(path string, urls []string) *boltStorage {
	db, err := bolt.Open(path, 0666, nil)
	if err != nil {
		panic(err)
	}

	// prepare buckets
	err = db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(latencyBucketName))
		if err != nil {
			return err
		}
		for _, url := range urls {
			// subbuckets (url -> latency)
			_, err := b.CreateBucketIfNotExists([]byte(url))
			if err != nil {
				return err
			}
		}

		_, err = tx.CreateBucketIfNotExists([]byte(avgLatencyBucketName))
		return err
	})
	if err != nil {
		panic(err)
	}

	return &boltStorage{
		path: path,
		db:   db,
	}
}

func (b *boltStorage) Close() error {
	return b.db.Close()
}

type avgItem struct {
	Count int64
	Sum   int64
	Avg   int64
}

func (b *boltStorage) PutLatency(url string, lat int64) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bnow := make([]byte, 8)
		binary.BigEndian.PutUint64(bnow, uint64(time.Now().UnixNano()))

		blat := make([]byte, 8)
		binary.BigEndian.PutUint64(blat, uint64(lat))

		latBkt := tx.Bucket([]byte(latencyBucketName)).Bucket([]byte(url))
		err := latBkt.Put(bnow, blat)
		if err != nil {
			return err
		}

		// reindex avg
		if lat >= 0 {
			avgBkt := tx.Bucket([]byte(avgLatencyBucketName))
			avg := avgBkt.Get([]byte(url))
			if avg == nil {
				item := &avgItem{1, lat, lat}
				bitem, err := json.Marshal(&item)
				if err != nil {
					return err
				}
				return avgBkt.Put([]byte(url), bitem)
			}

			item := avgItem{}
			err := json.Unmarshal(avg, &item)
			if err != nil {
				return err
			}
			item.Count++
			item.Sum += lat
			item.Avg = item.Sum / item.Count

			bitem, err := json.Marshal(&item)
			if err != nil {
				return err
			}
			return avgBkt.Put([]byte(url), bitem)
		}
		return nil
	})
}

func (b *boltStorage) GetMaxLatency() (string, int64, error) {
	var (
		max int64
		url string
	)
	err := b.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket([]byte(avgLatencyBucketName))
		c := bkt.Cursor()
		var item avgItem
		for burl, bitem := c.First(); burl != nil; burl, bitem = c.Next() {
			err := json.Unmarshal(bitem, &item)
			if err != nil {
				return err
			}
			if item.Avg > max {
				max = item.Avg
				url = string(burl)
			}
		}
		return nil
	})
	return url, max, err
}

func (b *boltStorage) GetMinLatency() (string, int64, error) {
	var (
		min int64 = -1
		url string
	)
	err := b.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket([]byte(avgLatencyBucketName))
		c := bkt.Cursor()
		var item avgItem
		for burl, bitem := c.First(); burl != nil; burl, bitem = c.Next() {
			err := json.Unmarshal(bitem, &item)
			if err != nil {
				return err
			}
			if min == -1 || item.Avg < min {
				min = item.Avg
				url = string(burl)
			}
		}
		return nil
	})
	return url, min, err
}

func (b *boltStorage) GetLastLatency(url string) (int64, error) {
	var last int64
	err := b.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket([]byte(latencyBucketName)).Bucket([]byte([]byte(url)))
		if bkt == nil {
			return errors.New("Not exists")
		}
		c := bkt.Cursor()
		bts, blat := c.Last()
		if bts == nil {
			return errors.New("Bucket is empty")
		}
		last = int64(binary.BigEndian.Uint64(blat))
		return nil
	})
	return last, err
}
