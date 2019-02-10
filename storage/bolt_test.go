package storage

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

const dbTestName = "test.db"

func TestLastLatency(t *testing.T) {
	urls := []string{"https://google.com", "https://notexist.eu"}
	bolt := NewBoltStorage(dbTestName, urls)
	defer cleanup(bolt, t)

	// https://google.com
	err := bolt.PutLatency(urls[0], 100000)
	assert.Nil(t, err, "PutLatency")
	err = bolt.PutLatency(urls[0], 200000)
	assert.Nil(t, err, "PutLatency")
	err = bolt.PutLatency(urls[0], 300000)
	assert.Nil(t, err, "PutLatency")

	// https://notexist.eu
	err = bolt.PutLatency(urls[1], 400000)
	assert.Nil(t, err, "PutLatency")
	err = bolt.PutLatency(urls[1], 500000)
	assert.Nil(t, err, "PutLatency")
	err = bolt.PutLatency(urls[1], 5000)
	assert.Nil(t, err, "PutLatency")

	lat, err := bolt.GetLastLatency(urls[0])
	assert.Nil(t, err, "GetLastLatency")
	assert.Equal(t, int64(300000), lat)

	lat, err = bolt.GetLastLatency(urls[1])
	assert.Nil(t, err, "GetLastLatency")
	assert.Equal(t, int64(5000), lat)

	// not available
	err = bolt.PutLatency(urls[1], -1)
	assert.Nil(t, err, "PutLatency")
	lat, err = bolt.GetLastLatency(urls[1])
	assert.Nil(t, err, "GetLastLatency")
	assert.Equal(t, int64(-1), lat)

	// available
	err = bolt.PutLatency(urls[1], 500)
	assert.Nil(t, err, "PutLatency")
	lat, err = bolt.GetLastLatency(urls[1])
	assert.Nil(t, err, "GetLastLatency")
	assert.Equal(t, int64(500), lat)
}

func TestMinMaxLatency(t *testing.T) {
	urls := []string{"https://google.com", "https://notexist.eu"}
	bolt := NewBoltStorage(dbTestName, urls)
	defer cleanup(bolt, t)

	// для https://google.com среднее значение 600 / 3 = 200
	err := bolt.PutLatency(urls[0], 100)
	assert.Nil(t, err, "PutLatency")
	err = bolt.PutLatency(urls[0], 200)
	assert.Nil(t, err, "PutLatency")
	err = bolt.PutLatency(urls[0], 300)
	assert.Nil(t, err, "PutLatency")
	err = bolt.PutLatency(urls[0], -1) // факт недоступности не учитывается
	assert.Nil(t, err, "PutLatency")

	// для https://notexist.eu среднее значение 300 / 3 = 100
	err = bolt.PutLatency(urls[0], -1)
	assert.Nil(t, err, "PutLatency")
	err = bolt.PutLatency(urls[1], 50)
	assert.Nil(t, err, "PutLatency")
	err = bolt.PutLatency(urls[1], 150)
	assert.Nil(t, err, "PutLatency")
	err = bolt.PutLatency(urls[1], 100)
	assert.Nil(t, err, "PutLatency")

	// минимальное среднее значине должно быть 100, а максимальное у гугла 200
	minURL, minLat, err := bolt.GetMinLatency()
	assert.Nil(t, err)
	assert.Equal(t, "https://notexist.eu", minURL)
	assert.Equal(t, int64(100), minLat)

	maxURL, maxLat, err := bolt.GetMaxLatency()
	assert.Nil(t, err)
	assert.Equal(t, "https://google.com", maxURL)
	assert.Equal(t, int64(200), maxLat)
}

func cleanup(bolt *boltStorage, t *testing.T) {
	err := bolt.Close()
	assert.Nil(t, err, "Cant close db")

	err = os.Remove(dbTestName)
	assert.Nil(t, err, "Cant remove db")
}
