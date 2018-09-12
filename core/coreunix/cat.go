package coreunix

import (
	"context"

	core "github.com/ipfs/go-ipfs/core"
	uio "gx/ipfs/QmPXzQ9LAFGZjcifFANCQFQiYt5SXgJziGoxUfJULVpHyA/go-unixfs/io"
	path "gx/ipfs/QmRYx6fJzTWFoeTo3qQn64iDrVC154Gy9waQDhvKRr2ND3/go-path"
	resolver "gx/ipfs/QmRYx6fJzTWFoeTo3qQn64iDrVC154Gy9waQDhvKRr2ND3/go-path/resolver"
)

func Cat(ctx context.Context, n *core.IpfsNode, pstr string) (uio.DagReader, error) {
	r := &resolver.Resolver{
		DAG:         n.DAG,
		ResolveOnce: uio.ResolveUnixfsOnce,
	}

	dagNode, err := core.Resolve(ctx, n.Namesys, r, path.Path(pstr))
	if err != nil {
		return nil, err
	}

	return uio.NewDagReader(ctx, dagNode, n.DAG)
}
