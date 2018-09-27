package namesys

import (
	"context"
	"strings"
	"time"

	opts "github.com/ipfs/go-ipfs/namesys/opts"
	path "gx/ipfs/QmcjwUb36Z16NJkvDX6ccXPqsFswo6AsRXynyXcLLCphV2/go-path"

	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	ci "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	lru "gx/ipfs/QmQjMHF8ptRgx4E57UFMiT4YM6kqaJeYxZ1MCDX23aw4rK/golang-lru"
	ds "gx/ipfs/QmUyz7JTJzgegC6tiJrfby3mPhzcdswVtG4x58TQ6pq8jV/go-datastore"
	routing "gx/ipfs/QmVBnJDKhtFXTRVjXKinqpwGu8t1DyNqPKan2iGX8PR8xG/go-libp2p-routing"
	isd "gx/ipfs/QmZmmuAXgX73UQmX1jRKjTGmjzq24Jinqkq8vzkBtno4uX/go-is-domain"
	peer "gx/ipfs/QmbNepETomvmXfz1X5pHNFD2QuPqnqi47dTd94QJWSorQ3/go-libp2p-peer"
)

// mpns (a multi-protocol NameSystem) implements generic IPFS naming.
//
// Uses several Resolvers:
// (a) IPFS routing naming: SFS-like PKI names.
// (b) dns domains: resolves using links in DNS TXT records
// (c) proquints: interprets string as the raw byte data.
//
// It can only publish to: (a) IPFS routing naming.
//
type mpns struct {
	dnsResolver, proquintResolver, ipnsResolver resolver
	ipnsPublisher                               Publisher

	cache *lru.Cache
}

// NewNameSystem will construct the IPFS naming system based on Routing
func NewNameSystem(r routing.ValueStore, ds ds.Datastore, cachesize int) NameSystem {
	var cache *lru.Cache
	if cachesize > 0 {
		cache, _ = lru.New(cachesize)
	}

	return &mpns{
		dnsResolver:      NewDNSResolver(),
		proquintResolver: new(ProquintResolver),
		ipnsResolver:     NewIpnsResolver(r),
		ipnsPublisher:    NewIpnsPublisher(r, ds),
		cache:            cache,
	}
}

const DefaultResolverCacheTTL = time.Minute

// Resolve implements Resolver.
func (ns *mpns) Resolve(ctx context.Context, name string, options ...opts.ResolveOpt) (path.Path, error) {
	if strings.HasPrefix(name, "/ipfs/") {
		return path.ParsePath(name)
	}

	if !strings.HasPrefix(name, "/") {
		return path.ParsePath("/ipfs/" + name)
	}

	return resolve(ctx, ns, name, opts.ProcessOpts(options), "/ipns/")
}

func (ns *mpns) ResolveAsync(ctx context.Context, name string, options ...opts.ResolveOpt) <-chan Result {
	res := make(chan Result, 1)
	if strings.HasPrefix(name, "/ipfs/") {
		p, err := path.ParsePath(name)
		res <- Result{p, err}
		return res
	}

	if !strings.HasPrefix(name, "/") {
		p, err := path.ParsePath("/ipfs/" + name)
		res <- Result{p, err}
		return res
	}

	return resolveAsync(ctx, ns, name, opts.ProcessOpts(options), "/ipns/")
}

// resolveOnce implements resolver.
func (ns *mpns) resolveOnceAsync(ctx context.Context, name string, options opts.ResolveOpts) <-chan onceResult {
	out := make(chan onceResult, 1)

	if !strings.HasPrefix(name, "/ipns/") {
		name = "/ipns/" + name
	}
	segments := strings.SplitN(name, "/", 4)
	if len(segments) < 3 || segments[0] != "" {
		log.Debugf("invalid name syntax for %s", name)
		out <- onceResult{err: ErrResolveFailed}
		close(out)
		return out
	}

	key := segments[2]

	if p, ok := ns.cacheGet(key); ok {
		out <- onceResult{value: p}
		close(out)
		return out
	}

	// Resolver selection:
	// 1. if it is a multihash resolve through "ipns".
	// 2. if it is a domain name, resolve through "dns"
	// 3. otherwise resolve through the "proquint" resolver

	var res resolver
	if _, err := mh.FromB58String(key); err == nil {
		res = ns.ipnsResolver
	} else if isd.IsDomain(key) {
		res = ns.dnsResolver
	} else {
		res = ns.proquintResolver
	}

	resCh := res.resolveOnceAsync(ctx, key, options)
	var best onceResult
	go func() {
		defer close(out)
		for {
			select {
			case res, ok := <-resCh:
				if !ok {
					if best != (onceResult{}) {
						ns.cacheSet(key, best.value, best.ttl)
					}
					return
				}
				if res.err == nil {
					best = res
				}
				p := res.value

				// Attach rest of the path
				if len(segments) > 3 {
					p, _ = path.FromSegments("", strings.TrimRight(p.String(), "/"), segments[3])
				}

				out <- onceResult{value: p, err: res.err}
			case <-ctx.Done():
				return
			}
		}
	}()

	return out
}

// Publish implements Publisher
func (ns *mpns) Publish(ctx context.Context, name ci.PrivKey, value path.Path) error {
	return ns.PublishWithEOL(ctx, name, value, time.Now().Add(DefaultRecordTTL))
}

func (ns *mpns) PublishWithEOL(ctx context.Context, name ci.PrivKey, value path.Path, eol time.Time) error {
	id, err := peer.IDFromPrivateKey(name)
	if err != nil {
		return err
	}
	if err := ns.ipnsPublisher.PublishWithEOL(ctx, name, value, eol); err != nil {
		return err
	}
	ttl := DefaultResolverCacheTTL
	if ttEol := eol.Sub(time.Now()); ttEol < ttl {
		ttl = ttEol
	}
	ns.cacheSet(peer.IDB58Encode(id), value, ttl)
	return nil
}
