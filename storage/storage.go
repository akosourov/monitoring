package storage

type Storage interface {
	PutLatency(url string, lat int64) error
	GetMaxLatency() (string, int64, error)
	GetMinLatency() (string, int64, error)
	GetLastLatency(url string) (int64, error)
	Close() error
}

func New(path string) Storage {
	return NewBoltStorage(path)
}
