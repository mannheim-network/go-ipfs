package coreunix

import (
	core "github.com/ipfs/go-ipfs/core"
	ft "gx/ipfs/QmVxjT67BU1QZUPzSLNZT6DkDzVNfPfkzqNyJYFXxSH2hA/go-unixfs"
	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	dag "gx/ipfs/QmfKKGzisaoP4oiHQSHz1zLbXDCTeXe7NVfX1FAMKzcHmt/go-merkledag"
)

func AddMetadataTo(n *core.IpfsNode, skey string, m *ft.Metadata) (string, error) {
	c, err := cid.Decode(skey)
	if err != nil {
		return "", err
	}

	nd, err := n.DAG.Get(n.Context(), c)
	if err != nil {
		return "", err
	}

	mdnode := new(dag.ProtoNode)
	mdata, err := ft.BytesForMetadata(m)
	if err != nil {
		return "", err
	}

	mdnode.SetData(mdata)
	if err := mdnode.AddNodeLink("file", nd); err != nil {
		return "", err
	}

	err = n.DAG.Add(n.Context(), mdnode)
	if err != nil {
		return "", err
	}

	return mdnode.Cid().String(), nil
}

func Metadata(n *core.IpfsNode, skey string) (*ft.Metadata, error) {
	c, err := cid.Decode(skey)
	if err != nil {
		return nil, err
	}

	nd, err := n.DAG.Get(n.Context(), c)
	if err != nil {
		return nil, err
	}

	pbnd, ok := nd.(*dag.ProtoNode)
	if !ok {
		return nil, dag.ErrNotProtobuf
	}

	return ft.MetadataFromBytes(pbnd.Data())
}
