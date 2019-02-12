/*
Реализация хранилища с помощью boltdb.

Схема данных:
1. Для хранения времени откликов для каждого url сервиса, создаем отдельный бакет,
в котором ключ - timestamp (нс), а значение - время отклика (нс). В случае, если
сервис недоступен значение будет равно -1.

2. "AvgLatency" - бакет, для хранения среднего (p50) значения времени отклика (нс) сервиса,
где	ключ - url сервиса, значение - json вида {Count: int, Sum: int64, Avg: int64},
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

const avgLatencyBucketName = "AvgLatency"

// BoltStorage реализует интерфейс Storage
type BoltStorage struct {
	path string
	db   *bolt.DB
}

// NewBoltStorage создает файл с базой данной и/или выполняет подключение к нему.
func NewBoltStorage(path string) *BoltStorage {
	db, err := bolt.Open(path, 0666, &bolt.Options{Timeout: 3 * time.Second})
	if err != nil {
		panic(err)
	}

	// prepare buckets
	err = db.Update(func(tx *bolt.Tx) error {
		_, err = tx.CreateBucketIfNotExists([]byte(avgLatencyBucketName))
		return err
	})
	if err != nil {
		panic(err)
	}

	return &BoltStorage{
		path: path,
		db:   db,
	}
}

func (b *BoltStorage) Close() error {
	return b.db.Close()
}

type avgItem struct {
	Count int64
	Sum   int64
	Avg   int64
}

func (b *BoltStorage) PutLatency(url string, lat int64) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bnow := make([]byte, 8)
		binary.BigEndian.PutUint64(bnow, uint64(time.Now().UnixNano()))

		blat := make([]byte, 8)
		binary.BigEndian.PutUint64(blat, uint64(lat))

		// если первое обращение, создаем бакет для хранения времени отклика для url
		latBkt, err := tx.CreateBucketIfNotExists([]byte(url))
		if err != nil {
			return err
		}

		if err := latBkt.Put(bnow, blat); err != nil {
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

func (b *BoltStorage) GetMaxLatency() (string, int64, error) {
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

func (b *BoltStorage) GetMinLatency() (string, int64, error) {
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

func (b *BoltStorage) GetLastLatency(url string) (int64, error) {
	var last int64
	err := b.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket([]byte(url))
		if bkt == nil {
			return errors.New(url + " not exists")
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

func (b *BoltStorage) GetAvgLatency(url string) (int64, error) {
	var avg int64
	err := b.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket([]byte(avgLatencyBucketName))
		bitem := bkt.Get([]byte(url))
		if bitem == nil {
			return errors.New(url + "not exists")
		}
		var item avgItem
		if err := json.Unmarshal(bitem, &item); err != nil {
			return err
		}
		avg = item.Avg
		return nil
	})
	return avg, err
}
