package coreapi

import (
	context "context"
	fmt "fmt"
	gopath "path"

	core "github.com/ipfs/go-ipfs/core"
	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	namesys "github.com/ipfs/go-ipfs/namesys"
	ipfspath "gx/ipfs/QmTG5WFmAM4uAnqGskeAPijdpTmmNDLJNCQ71NqfdvC6hV/go-path"
	resolver "gx/ipfs/QmTG5WFmAM4uAnqGskeAPijdpTmmNDLJNCQ71NqfdvC6hV/go-path/resolver"
	uio "gx/ipfs/QmTbas51oodp3ZJrqsWYs1yqSxcD7LEJBv4djRV2VrY8wv/go-unixfs/io"

	ipld "gx/ipfs/QmUSyMZ8Vt4vTZr5HdDEgEfpwAXfQRuDdfCFTt7XBzhxpQ/go-ipld-format"
	cid "gx/ipfs/Qmdu2AYUV7yMoVBQPxXNfe7FJcdx16kYtsx6jAPKWQYF1y/go-cid"
)

// ResolveNode resolves the path `p` using Unixfs resolver, gets and returns the
// resolved Node.
func (api *CoreAPI) ResolveNode(ctx context.Context, p coreiface.Path) (ipld.Node, error) {
	return resolveNode(ctx, api.node.DAG, api.node.Namesys, p)
}

// ResolvePath resolves the path `p` using Unixfs resolver, returns the
// resolved path.
func (api *CoreAPI) ResolvePath(ctx context.Context, p coreiface.Path) (coreiface.ResolvedPath, error) {
	return resolvePath(ctx, api.node.DAG, api.node.Namesys, p)
}

func resolveNode(ctx context.Context, ng ipld.NodeGetter, nsys namesys.NameSystem, p coreiface.Path) (ipld.Node, error) {
	rp, err := resolvePath(ctx, ng, nsys, p)
	if err != nil {
		return nil, err
	}

	node, err := ng.Get(ctx, rp.Cid())
	if err != nil {
		return nil, err
	}
	return node, nil
}

func resolvePath(ctx context.Context, ng ipld.NodeGetter, nsys namesys.NameSystem, p coreiface.Path) (coreiface.ResolvedPath, error) {
	if _, ok := p.(coreiface.ResolvedPath); ok {
		return p.(coreiface.ResolvedPath), nil
	}

	ipath := ipfspath.Path(p.String())
	ipath, err := core.ResolveIPNS(ctx, nsys, ipath)
	if err == core.ErrNoNamesys {
		return nil, coreiface.ErrOffline
	} else if err != nil {
		return nil, err
	}

	var resolveOnce resolver.ResolveOnce

	switch ipath.Segments()[0] {
	case "ipfs":
		resolveOnce = uio.ResolveUnixfsOnce
	case "ipld":
		resolveOnce = resolver.ResolveSingle
	default:
		return nil, fmt.Errorf("unsupported path namespace: %s", p.Namespace())
	}

	r := &resolver.Resolver{
		DAG:         ng,
		ResolveOnce: resolveOnce,
	}

	node, rest, err := r.ResolveToLastNode(ctx, ipath)
	if err != nil {
		return nil, err
	}

	root, err := cid.Parse(ipath.Segments()[1])
	if err != nil {
		return nil, err
	}

	return coreiface.NewResolvedPath(ipath, node, root, gopath.Join(rest...)), nil
}
