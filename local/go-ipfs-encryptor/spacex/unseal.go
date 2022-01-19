package spacex

import (
	"fmt"

	"github.com/dgraph-io/badger"
)

func Unseal(path string) ([]byte, error, int) {
	return Worker.unseal(path)
}

func GetSize(item *badger.Item) (int, error) {
	value, err := item.ValueCopy(nil)
	if err != nil {
		return -1, err
	}

	if ok, si := TryGetSealedInfo(value); ok {
		if len(si.Sbs) == 0 {
			return -1, fmt.Errorf("Sbs is empty, can't get block size")
		}
		return si.Sbs[0].Size, nil
	}

	return len(value), nil
}
