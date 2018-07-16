package io

import (
	"context"
	"errors"
	"fmt"
	"io"

	mdag "github.com/ipfs/go-ipfs/merkledag"
	ft "github.com/ipfs/go-ipfs/unixfs"
	ftpb "github.com/ipfs/go-ipfs/unixfs/pb"

	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	ipld "gx/ipfs/QmZtNq8dArGfnpCZfx2pUNY7UcjGhVp5qqwQ4hH6mpTMRQ/go-ipld-format"
)

// PBDagReader provides a way to easily read the data contained in a dag.
type PBDagReader struct {
	serv ipld.NodeGetter

	// UnixFS file (it should be of type `Data_File` or `Data_Raw` only).
	file *ft.FSNode

	// the current data buffer to be read from
	// will either be a bytes.Reader or a child DagReader
	buf ReadSeekCloser

	// NodePromises for each of 'nodes' child links
	promises []*ipld.NodePromise

	// the cid of each child of the current node
	links []*cid.Cid

	// the index of the child link currently being read from
	linkPosition int

	// current offset for the read head within the 'file'
	offset int64

	// Our context
	ctx context.Context

	// context cancel for children
	cancel func()
}

var _ DagReader = (*PBDagReader)(nil)

// NewPBFileReader constructs a new PBFileReader.
func NewPBFileReader(ctx context.Context, n *mdag.ProtoNode, file *ft.FSNode, serv ipld.NodeGetter) *PBDagReader {
	fctx, cancel := context.WithCancel(ctx)
	curLinks := getLinkCids(n)
	return &PBDagReader{
		serv:     serv,
		buf:      NewBufDagReader(file.Data()),
		promises: make([]*ipld.NodePromise, len(curLinks)),
		links:    curLinks,
		ctx:      fctx,
		cancel:   cancel,
		file:     file,
	}
}

const preloadSize = 10

func (dr *PBDagReader) preloadNextNodes(ctx context.Context) {
	beg := dr.linkPosition
	end := beg + preloadSize
	if end >= len(dr.links) {
		end = len(dr.links)
	}

	for i, p := range ipld.GetNodes(ctx, dr.serv, dr.links[beg:end]) {
		dr.promises[beg+i] = p
	}
}

// precalcNextBuf follows the next link in line and loads it from the
// DAGService, setting the next buffer to read from
func (dr *PBDagReader) precalcNextBuf(ctx context.Context) error {
	if dr.buf != nil {
		dr.buf.Close() // Just to make sure
		dr.buf = nil
	}

	if dr.linkPosition >= len(dr.promises) {
		return io.EOF
	}

	if dr.promises[dr.linkPosition] == nil {
		dr.preloadNextNodes(ctx)
	}

	nxt, err := dr.promises[dr.linkPosition].Get(ctx)
	if err != nil {
		return err
	}
	dr.promises[dr.linkPosition] = nil
	dr.linkPosition++

	switch nxt := nxt.(type) {
	case *mdag.ProtoNode:
		fsNode, err := ft.FSNodeFromBytes(nxt.Data())
		if err != nil {
			return fmt.Errorf("incorrectly formatted protobuf: %s", err)
		}

		switch fsNode.Type() {
		case ftpb.Data_Directory, ftpb.Data_HAMTShard:
			// A directory should not exist within a file
			return ft.ErrInvalidDirLocation
		case ftpb.Data_File:
			dr.buf = NewPBFileReader(dr.ctx, nxt, fsNode, dr.serv)
			return nil
		case ftpb.Data_Raw:
			dr.buf = NewBufDagReader(fsNode.Data())
			return nil
		case ftpb.Data_Metadata:
			return errors.New("shouldnt have had metadata object inside file")
		case ftpb.Data_Symlink:
			return errors.New("shouldnt have had symlink inside file")
		default:
			return ft.ErrUnrecognizedType
		}
	default:
		var err error
		dr.buf, err = NewDagReader(ctx, nxt, dr.serv)
		return err
	}
}

func getLinkCids(n ipld.Node) []*cid.Cid {
	links := n.Links()
	out := make([]*cid.Cid, 0, len(links))
	for _, l := range links {
		out = append(out, l.Cid)
	}
	return out
}

// Size return the total length of the data from the DAG structured file.
func (dr *PBDagReader) Size() uint64 {
	return dr.file.FileSize()
}

// Read reads data from the DAG structured file
func (dr *PBDagReader) Read(b []byte) (int, error) {
	return dr.CtxReadFull(dr.ctx, b)
}

// CtxReadFull reads data from the DAG structured file
func (dr *PBDagReader) CtxReadFull(ctx context.Context, b []byte) (int, error) {
	if dr.buf == nil {
		if err := dr.precalcNextBuf(ctx); err != nil {
			return 0, err
		}
	}

	// If no cached buffer, load one
	total := 0
	for {
		// Attempt to fill bytes from cached buffer
		n, err := io.ReadFull(dr.buf, b[total:])
		total += n
		dr.offset += int64(n)
		switch err {
		// io.EOF will happen is dr.buf had noting more to read (n == 0)
		case io.EOF, io.ErrUnexpectedEOF:
			// do nothing
		case nil:
			return total, nil
		default:
			return total, err
		}

		// if we are not done with the output buffer load next block
		err = dr.precalcNextBuf(ctx)
		if err != nil {
			return total, err
		}
	}
}

// WriteTo writes to the given writer.
func (dr *PBDagReader) WriteTo(w io.Writer) (int64, error) {
	if dr.buf == nil {
		if err := dr.precalcNextBuf(dr.ctx); err != nil {
			return 0, err
		}
	}

	// If no cached buffer, load one
	total := int64(0)
	for {
		// Attempt to write bytes from cached buffer
		n, err := dr.buf.WriteTo(w)
		total += n
		dr.offset += n
		if err != nil {
			if err != io.EOF {
				return total, err
			}
		}

		// Otherwise, load up the next block
		err = dr.precalcNextBuf(dr.ctx)
		if err != nil {
			if err == io.EOF {
				return total, nil
			}
			return total, err
		}
	}
}

// Close closes the reader.
func (dr *PBDagReader) Close() error {
	dr.cancel()
	return nil
}

// Seek implements io.Seeker, and will seek to a given offset in the file
// interface matches standard unix seek
// TODO: check if we can do relative seeks, to reduce the amount of dagreader
// recreations that need to happen.
func (dr *PBDagReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		if offset < 0 {
			return -1, errors.New("invalid offset")
		}
		if offset == dr.offset {
			return offset, nil
		}

		// left represents the number of bytes remaining to seek to (from beginning)
		left := offset
		if int64(len(dr.file.Data())) >= offset {
			// Close current buf to close potential child dagreader
			if dr.buf != nil {
				dr.buf.Close()
			}
			dr.buf = NewBufDagReader(dr.file.Data()[offset:])

			// start reading links from the beginning
			dr.linkPosition = 0
			dr.offset = offset
			return offset, nil
		}

		// skip past root block data
		left -= int64(len(dr.file.Data()))

		// iterate through links and find where we need to be
		for i := 0; i < dr.file.NumChildren(); i++ {
			if dr.file.BlockSize(i) > uint64(left) {
				dr.linkPosition = i
				break
			} else {
				left -= int64(dr.file.BlockSize(i))
			}
		}

		// start sub-block request
		err := dr.precalcNextBuf(dr.ctx)
		if err != nil {
			return 0, err
		}

		// set proper offset within child readseeker
		n, err := dr.buf.Seek(left, io.SeekStart)
		if err != nil {
			return -1, err
		}

		// sanity
		left -= n
		if left != 0 {
			return -1, errors.New("failed to seek properly")
		}
		dr.offset = offset
		return offset, nil
	case io.SeekCurrent:
		// TODO: be smarter here
		if offset == 0 {
			return dr.offset, nil
		}

		noffset := dr.offset + offset
		return dr.Seek(noffset, io.SeekStart)
	case io.SeekEnd:
		noffset := int64(dr.file.FileSize()) - offset
		n, err := dr.Seek(noffset, io.SeekStart)

		// Return negative number if we can't figure out the file size. Using io.EOF
		// for this seems to be good(-enough) solution as it's only returned by
		// precalcNextBuf when we step out of file range.
		// This is needed for gateway to function properly
		if err == io.EOF && dr.file.Type() == ftpb.Data_File {
			return -1, nil
		}
		return n, err
	default:
		return 0, errors.New("invalid whence")
	}
}
