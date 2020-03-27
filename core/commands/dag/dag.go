package dagcmd

import (
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	"github.com/ipfs/go-ipfs/core/coredag"
	mdag "github.com/ipfs/go-merkledag"

	cid "github.com/ipfs/go-cid"
	cidenc "github.com/ipfs/go-cidutil/cidenc"
	cmds "github.com/ipfs/go-ipfs-cmds"
	files "github.com/ipfs/go-ipfs-files"
	ipld "github.com/ipfs/go-ipld-format"
	ipfspath "github.com/ipfs/go-path"
	path "github.com/ipfs/interface-go-ipfs-core/path"
	mh "github.com/multiformats/go-multihash"

	gocar "github.com/ipld/go-car"
	//gipfree "github.com/ipld/go-ipld-prime/impl/free"
	//gipselector "github.com/ipld/go-ipld-prime/traversal/selector"
	//gipselectorbuilder "github.com/ipld/go-ipld-prime/traversal/selector/builder"
)

var DagCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with ipld dag objects.",
		ShortDescription: `
'ipfs dag' is used for creating and manipulating dag objects/hierarchies.

This subcommand is currently an experimental feature, but it is intended
to deprecate and replace the existing 'ipfs object' command moving forward.
		`,
	},
	Subcommands: map[string]*cmds.Command{
		"put":     DagPutCmd,
		"get":     DagGetCmd,
		"resolve": DagResolveCmd,
		"export":  DagExportCmd,
	},
}

// OutputObject is the output type of 'dag put' command
type OutputObject struct {
	Cid cid.Cid
}

// ResolveOutput is the output type of 'dag resolve' command
type ResolveOutput struct {
	Cid     cid.Cid
	RemPath string
}

var DagPutCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Add a dag node to ipfs.",
		ShortDescription: `
'ipfs dag put' accepts input from a file or stdin and parses it
into an object of the specified format.
`,
	},
	Arguments: []cmds.Argument{
		cmds.FileArg("object data", true, true, "The object to put").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption("format", "f", "Format that the object will be added as.").WithDefault("cbor"),
		cmds.StringOption("input-enc", "Format that the input object will be.").WithDefault("json"),
		cmds.BoolOption("pin", "Pin this object when adding."),
		cmds.StringOption("hash", "Hash function to use").WithDefault(""),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		ienc, _ := req.Options["input-enc"].(string)
		format, _ := req.Options["format"].(string)
		hash, _ := req.Options["hash"].(string)
		dopin, _ := req.Options["pin"].(bool)

		// mhType tells inputParser which hash should be used. MaxUint64 means 'use
		// default hash' (sha256 for cbor, sha1 for git..)
		mhType := uint64(math.MaxUint64)

		if hash != "" {
			var ok bool
			mhType, ok = mh.Names[hash]
			if !ok {
				return fmt.Errorf("%s in not a valid multihash name", hash)
			}
		}

		var adder ipld.NodeAdder = api.Dag()
		if dopin {
			adder = api.Dag().Pinning()
		}
		b := ipld.NewBatch(req.Context, adder)

		it := req.Files.Entries()
		for it.Next() {
			file := files.FileFromEntry(it)
			if file == nil {
				return fmt.Errorf("expected a regular file")
			}
			nds, err := coredag.ParseInputs(ienc, format, file, mhType, -1)
			if err != nil {
				return err
			}
			if len(nds) == 0 {
				return fmt.Errorf("no node returned from ParseInputs")
			}

			for _, nd := range nds {
				err := b.Add(req.Context, nd)
				if err != nil {
					return err
				}
			}

			cid := nds[0].Cid()
			if err := res.Emit(&OutputObject{Cid: cid}); err != nil {
				return err
			}
		}
		if it.Err() != nil {
			return it.Err()
		}

		if err := b.Commit(); err != nil {
			return err
		}

		return nil
	},
	Type: OutputObject{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *OutputObject) error {
			enc, err := cmdenv.GetLowLevelCidEncoder(req)
			if err != nil {
				return err
			}
			fmt.Fprintln(w, enc.Encode(out.Cid))
			return nil
		}),
	},
}

var DagGetCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get a dag node from ipfs.",
		ShortDescription: `
'ipfs dag get' fetches a dag node from ipfs and prints it out in the specified
format.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("ref", true, false, "The object to get").EnableStdin(),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		rp, err := api.ResolvePath(req.Context, path.New(req.Arguments[0]))
		if err != nil {
			return err
		}

		obj, err := api.Dag().Get(req.Context, rp.Cid())
		if err != nil {
			return err
		}

		var out interface{} = obj
		if len(rp.Remainder()) > 0 {
			rem := strings.Split(rp.Remainder(), "/")
			final, _, err := obj.Resolve(rem)
			if err != nil {
				return err
			}
			out = final
		}
		return cmds.EmitOnce(res, &out)
	},
}

// DagResolveCmd returns address of highest block within a path and a path remainder
var DagResolveCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Resolve ipld block",
		ShortDescription: `
'ipfs dag resolve' fetches a dag node from ipfs, prints it's address and remaining path.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("ref", true, false, "The path to resolve").EnableStdin(),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		rp, err := api.ResolvePath(req.Context, path.New(req.Arguments[0]))
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, &ResolveOutput{
			Cid:     rp.Cid(),
			RemPath: rp.Remainder(),
		})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *ResolveOutput) error {
			var (
				enc cidenc.Encoder
				err error
			)
			switch {
			case !cmdenv.CidBaseDefined(req):
				// Not specified, check the path.
				enc, err = cmdenv.CidEncoderFromPath(req.Arguments[0])
				if err == nil {
					break
				}
				// Nope, fallback on the default.
				fallthrough
			default:
				enc, err = cmdenv.GetLowLevelCidEncoder(req)
				if err != nil {
					return err
				}
			}
			p := enc.Encode(out.Cid)
			if out.RemPath != "" {
				p = ipfspath.Join([]string{p, out.RemPath})
			}

			fmt.Fprint(w, p)
			return nil
		}),
	},
	Type: ResolveOutput{},
}

var DagExportCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Streams the selected DAG as a .car stream on stdout.",
		ShortDescription: `
'ipfs dag export' fetches a dag and streams it out as a well-formed .car file.
Note that at present only single root selections / .car files are supported.
The output of blocks happens in strict DAG-traversal, first-seen, order.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("root", true, false, "CID of a root to recursively export").EnableStdin(),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {

		c, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return fmt.Errorf(
				"unable to parse root specification (currently only bare CIDs are supported): %s",
				err,
			)
		}

		node, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		// Code disabled until descent-issue in go-ipld-prime is fixed
		// https://github.com/ribasushi/gip-muddle-up
		//
		// sb := gipselectorbuilder.NewSelectorSpecBuilder(gipfree.NodeBuilder())
		// car := gocar.NewSelectiveCar(
		// 	req.Context,
		// 	<needs to be fixed to take format.NodeGetter as well>,
		// 	[]gocar.Dag{gocar.Dag{
		// 		Root: c,
		// 		Selector: sb.ExploreRecursive(
		// 			gipselector.RecursionLimitNone(),
		// 			sb.ExploreAll(sb.ExploreRecursiveEdge()),
		// 		).Node(),
		// 	}},
		// )

		pipeR, pipeW := io.Pipe()

		errCh := make(chan error, 2) // we only report the 1st error
		go func() {
			defer func() {
				if err := pipeW.Close(); err != nil {
					errCh <- fmt.Errorf("stream flush failed: %s", err)
				}
				close(errCh)
			}()

			//if err := car.Write(pipeW); err != nil {
			if err := gocar.WriteCar(
				req.Context,
				mdag.NewSession(
					req.Context,
					node.DAG,
				),
				[]cid.Cid{c},
				pipeW,
			); err != nil {
				errCh <- err
			}
		}()

		if err := res.Emit(pipeR); err != nil {
			return err
		}

		err = <-errCh

		// minimal user friendliness
		if err != nil &&
			!node.IsOnline &&
			err.Error() == "merkledag: not found" {
			err = fmt.Errorf("%s (currently offline, perhaps retry after attaching to the network)", err)
		}

		return err
	},
}
