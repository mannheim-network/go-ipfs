package spacex

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"
)

const sealContextKey string = "sckey"

func GenSealContext(ctx context.Context, root cid.Cid) context.Context {
	return context.WithValue(ctx, sealContextKey, root.String())
}

func GetRootFromSealContext(ctx context.Context) (cid.Cid, error) {
	v := ctx.Value(sealContextKey)
	if v == nil {
		return cid.Undef, fmt.Errorf("Can't find root cid from context")
	}
	if buf, ok := v.(string); ok {
		return cid.Parse(buf)
	}
	return cid.Undef, fmt.Errorf("Can't find root cid from context")
}

func GetStoreFlag(root cid.Cid, blockCid cid.Cid) bool {
	return sealedMap.blockExist(root, blockCid)
}

func SetStoreFlag(root cid.Cid, blockCid cid.Cid) {
	sealedMap.addBlock(root, blockCid)
}
