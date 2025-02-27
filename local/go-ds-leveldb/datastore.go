package leveldb

import (
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	spacex "github.com/mannheim-network/go-ipfs-encryptor/spacex"
	"github.com/ipfs/go-datastore"
	ds "github.com/ipfs/go-datastore"
	dsq "github.com/ipfs/go-datastore/query"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/storage"
	"github.com/syndtr/goleveldb/leveldb/util"
)

type Datastore struct {
	*accessor
	DB   *leveldb.DB
	path string
}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

var _ ds.Datastore = (*Datastore)(nil)
var _ ds.TxnDatastore = (*Datastore)(nil)

// Options is an alias of syndtr/goleveldb/opt.Options which might be extended
// in the future.
type Options opt.Options

// NewDatastore returns a new datastore backed by leveldb
//
// for path == "", an in memory bachend will be chosen
func NewDatastore(path string, opts *Options) (*Datastore, error) {
	var nopts opt.Options
	if opts != nil {
		nopts = opt.Options(*opts)
	}

	var err error
	var db *leveldb.DB

	if path == "" {
		db, err = leveldb.Open(storage.NewMemStorage(), &nopts)
	} else {
		db, err = leveldb.OpenFile(path, &nopts)
		if errors.IsCorrupted(err) && !nopts.GetReadOnly() {
			db, err = leveldb.RecoverFile(path, &nopts)
		}
	}

	if err != nil {
		return nil, err
	}

	ds := Datastore{
		accessor: &accessor{ldb: db, syncWrites: true, closeLk: new(sync.RWMutex)},
		DB:       db,
		path:     path,
	}
	return &ds, nil
}

// An extraction of the common interface between LevelDB Transactions and the DB itself.
//
// It allows to plug in either inside the `accessor`.
type levelDbOps interface {
	Put(key, value []byte, wo *opt.WriteOptions) error
	Get(key []byte, ro *opt.ReadOptions) (value []byte, err error)
	Has(key []byte, ro *opt.ReadOptions) (ret bool, err error)
	Delete(key []byte, wo *opt.WriteOptions) error
	NewIterator(slice *util.Range, ro *opt.ReadOptions) iterator.Iterator
}

// Datastore operations using either the DB or a transaction as the backend.
type accessor struct {
	ldb        levelDbOps
	syncWrites bool
	closeLk    *sync.RWMutex
}

func (a *accessor) Put(key ds.Key, value []byte) (err error) {
	a.closeLk.RLock()
	defer a.closeLk.RUnlock()

	if ok, sb := spacex.TryGetSealedBlock(value); ok {
		// fmt.Printf("Sb: {path: %s, size: %d}\n", sb.Path, sb.Size)
		data, err := a.GetRaw(key)
		if err == datastore.ErrNotFound {
			return a.ldb.Put(key.Bytes(), sb.ToSealedInfo().Bytes(), &opt.WriteOptions{Sync: a.syncWrites})
		} else if err != nil {
			return err
		}

		if ok, si := spacex.TryGetSealedInfo(data); !ok {
			return a.ldb.Put(key.Bytes(), sb.ToSealedInfo().Bytes(), &opt.WriteOptions{Sync: a.syncWrites})
		} else {
			// for i := 0; i < len(si.Sbs); i++ {
			// fmt.Printf("Sbs[%d]: {path: %s, size: %d}\n", i, si.Sbs[i].Path, si.Sbs[i].Size)
			// }
			return a.ldb.Put(key.Bytes(), si.AddSealedBlock(*sb).Bytes(), &opt.WriteOptions{Sync: a.syncWrites})
		}
	}

	return a.ldb.Put(key.Bytes(), value, &opt.WriteOptions{Sync: a.syncWrites})
}

func (a *accessor) Sync(prefix ds.Key) error {
	return nil
}

func (a *accessor) GetRaw(key ds.Key) (value []byte, err error) {
	a.closeLk.RLock()
	defer a.closeLk.RUnlock()
	val, err := a.ldb.Get(key.Bytes(), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil, ds.ErrNotFound
		}
		return nil, err
	}
	return val, nil
}

func (a *accessor) Get(key ds.Key) (value []byte, err error) {
	a.closeLk.RLock()
	defer a.closeLk.RUnlock()
	val, err := a.ldb.Get(key.Bytes(), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil, ds.ErrNotFound
		}
		return nil, err
	}

	if ok, si := spacex.TryGetSealedInfo(val); ok {
		sbsLen := len(si.Sbs)
		if sbsLen == 0 {
			return nil, ds.ErrNotFound
		}

		rindex := rand.Intn(sbsLen)

		var ret []byte
		var code int
		for i := rindex; i < len(si.Sbs)+rindex; {
			nowi := i % len(si.Sbs)
			ret, err, code = spacex.Unseal(si.Sbs[nowi].Path)
			if err != nil {
				switch code {
				case 200:
					return ret, nil
				// Can't find
				case 404:
					si.Sbs = append(si.Sbs[:nowi], si.Sbs[nowi+1:]...)
					continue
				// Lost
				case 410:
					i++
					continue
				default:
					return nil, err
				}
			} else {
				break
			}
		}

		if sbsLen != len(si.Sbs) {
			err = a.Put(key, si.Bytes())
			if err != nil {
				return nil, err
			}
		}

		if ret == nil {
			return nil, ds.ErrNotFound
		} else {
			return ret, nil
		}
	}

	return val, nil
}

func (a *accessor) Has(key ds.Key) (exists bool, err error) {
	a.closeLk.RLock()
	defer a.closeLk.RUnlock()
	return a.ldb.Has(key.Bytes(), nil)
}

func (a *accessor) GetSize(key ds.Key) (size int, err error) {
	value, err := a.GetRaw(key)
	if err != nil {
		return -1, err
	}

	if ok, si := spacex.TryGetSealedInfo(value); ok {
		if len(si.Sbs) == 0 {
			return -1, ds.ErrNotFound
		}
		return si.Sbs[0].Size, nil
	}

	return len(value), nil
}

func (a *accessor) Delete(key ds.Key) (err error) {
	a.closeLk.RLock()
	defer a.closeLk.RUnlock()
	return a.ldb.Delete(key.Bytes(), &opt.WriteOptions{Sync: a.syncWrites})
}

func (a *accessor) Query(q dsq.Query) (dsq.Results, error) {
	a.closeLk.RLock()
	defer a.closeLk.RUnlock()
	var rnge *util.Range

	// make a copy of the query for the fallback naive query implementation.
	// don't modify the original so res.Query() returns the correct results.
	qNaive := q
	prefix := ds.NewKey(q.Prefix).String()
	if prefix != "/" {
		rnge = util.BytesPrefix([]byte(prefix + "/"))
		qNaive.Prefix = ""
	}
	i := a.ldb.NewIterator(rnge, nil)
	next := i.Next
	if len(q.Orders) > 0 {
		switch q.Orders[0].(type) {
		case dsq.OrderByKey, *dsq.OrderByKey:
			qNaive.Orders = nil
		case dsq.OrderByKeyDescending, *dsq.OrderByKeyDescending:
			next = func() bool {
				next = i.Prev
				return i.Last()
			}
			qNaive.Orders = nil
		default:
		}
	}
	r := dsq.ResultsFromIterator(q, dsq.Iterator{
		Next: func() (dsq.Result, bool) {
			a.closeLk.RLock()
			defer a.closeLk.RUnlock()
			if !next() {
				return dsq.Result{}, false
			}
			k := string(i.Key())
			e := dsq.Entry{Key: k, Size: len(i.Value())}

			if !q.KeysOnly {
				buf := make([]byte, len(i.Value()))
				copy(buf, i.Value())
				e.Value = buf
			}
			return dsq.Result{Entry: e}, true
		},
		Close: func() error {
			a.closeLk.RLock()
			defer a.closeLk.RUnlock()
			i.Release()
			return nil
		},
	})
	return dsq.NaiveQueryApply(qNaive, r), nil
}

// DiskUsage returns the current disk size used by this levelDB.
// For in-mem datastores, it will return 0.
func (d *Datastore) DiskUsage() (uint64, error) {
	d.closeLk.RLock()
	defer d.closeLk.RUnlock()
	if d.path == "" { // in-mem
		return 0, nil
	}

	var du uint64

	err := filepath.Walk(d.path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		du += uint64(info.Size())
		return nil
	})

	if err != nil {
		return 0, err
	}

	return du, nil
}

// LevelDB needs to be closed.
func (d *Datastore) Close() (err error) {
	d.closeLk.Lock()
	defer d.closeLk.Unlock()
	return d.DB.Close()
}

type leveldbBatch struct {
	b          *leveldb.Batch
	db         *leveldb.DB
	closeLk    *sync.RWMutex
	syncWrites bool
}

func (d *Datastore) Batch() (ds.Batch, error) {
	return &leveldbBatch{
		b:          new(leveldb.Batch),
		db:         d.DB,
		closeLk:    d.closeLk,
		syncWrites: d.syncWrites,
	}, nil
}

func (b *leveldbBatch) Put(key ds.Key, value []byte) error {
	b.b.Put(key.Bytes(), value)
	return nil
}

func (b *leveldbBatch) Commit() error {
	b.closeLk.RLock()
	defer b.closeLk.RUnlock()
	return b.db.Write(b.b, &opt.WriteOptions{Sync: b.syncWrites})
}

func (b *leveldbBatch) Delete(key ds.Key) error {
	b.b.Delete(key.Bytes())
	return nil
}

// A leveldb transaction embedding the accessor backed by the transaction.
type transaction struct {
	*accessor
	tx *leveldb.Transaction
}

func (t *transaction) Commit() error {
	t.closeLk.RLock()
	defer t.closeLk.RUnlock()
	return t.tx.Commit()
}

func (t *transaction) Discard() {
	t.closeLk.RLock()
	defer t.closeLk.RUnlock()
	t.tx.Discard()
}

func (d *Datastore) NewTransaction(readOnly bool) (ds.Txn, error) {
	d.closeLk.RLock()
	defer d.closeLk.RUnlock()
	tx, err := d.DB.OpenTransaction()
	if err != nil {
		return nil, err
	}
	accessor := &accessor{ldb: tx, syncWrites: false, closeLk: d.closeLk}
	return &transaction{accessor, tx}, nil
}
