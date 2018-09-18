package commands

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ipfs/go-ipfs/core"
	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	ncmd "github.com/ipfs/go-ipfs/core/commands/name"
	ns "github.com/ipfs/go-ipfs/namesys"
	nsopts "github.com/ipfs/go-ipfs/namesys/opts"
	path "gx/ipfs/QmX7uSbkNz76yNwBhuwYwRbhihLnJqM73VTCjS3UMJud9A/go-path"

	uio "gx/ipfs/QmPL8bYtbACcSFFiSr4s2du7Na382NxRADR8hC7D9FkEA2/go-unixfs/io"
	"gx/ipfs/QmPTfgFTo9PFr1PvPKyKoeMgBvYPh6cX3aDP7DHKVbnCbi/go-ipfs-cmds"
	"gx/ipfs/QmSP88ryZkHSRn1fnngAaV2Vcn63WUJzAavnRM9CVdU1Ky/go-ipfs-cmdkit"
	resolver "gx/ipfs/QmX7uSbkNz76yNwBhuwYwRbhihLnJqM73VTCjS3UMJud9A/go-path/resolver"
)

var ResolveCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Resolve the value of names to IPFS.",
		ShortDescription: `
There are a number of mutable name protocols that can link among
themselves and into IPNS. This command accepts any of these
identifiers and resolves them to the referenced item.
`,
		LongDescription: `
There are a number of mutable name protocols that can link among
themselves and into IPNS. For example IPNS references can (currently)
point at an IPFS object, and DNS links can point at other DNS links, IPNS
entries, or IPFS objects. This command accepts any of these
identifiers and resolves them to the referenced item.

EXAMPLES

Resolve the value of your identity:

  $ ipfs resolve /ipns/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy
  /ipfs/Qmcqtw8FfrVSBaRmbWwHxt3AuySBhJLcvmFYi3Lbc4xnwj

Resolve the value of another name:

  $ ipfs resolve /ipns/QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n
  /ipns/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy

Resolve the value of another name recursively:

  $ ipfs resolve -r /ipns/QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n
  /ipfs/Qmcqtw8FfrVSBaRmbWwHxt3AuySBhJLcvmFYi3Lbc4xnwj

Resolve the value of an IPFS DAG path:

  $ ipfs resolve /ipfs/QmeZy1fGbwgVSrqbfh9fKQrAWgeyRnj7h8fsHS1oy3k99x/beep/boop
  /ipfs/QmYRMjyvAiHKN9UTi8Bzt1HUspmSRD8T8DwxfSMzLgBon1

`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("name", true, false, "The name to resolve.").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("recursive", "r", "Resolve until the result is an IPFS name."),
		cmdkit.UintOption("dht-record-count", "dhtrc", "Number of records to request for DHT resolution."),
		cmdkit.StringOption("dht-timeout", "dhtt", "Max time to collect values during DHT resolution eg \"30s\". Pass 0 for no timeout."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) {
		n, err := cmdenv.GetNode(env)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		name := req.Arguments[0]
		recursive, _ := req.Options["recursive"].(bool)

		// the case when ipns is resolved step by step
		if strings.HasPrefix(name, "/ipns/") && !recursive {
			rc, rcok := req.Options["dht-record-count"].(int)
			dhtt, dhttok := req.Options["dht-timeout"].(string)
			ropts := []nsopts.ResolveOpt{nsopts.Depth(1)}
			if rcok {
				ropts = append(ropts, nsopts.DhtRecordCount(uint(rc)))
			}
			if dhttok {
				d, err := time.ParseDuration(dhtt)
				if err != nil {
					res.SetError(err, cmdkit.ErrNormal)
					return
				}
				if d < 0 {
					res.SetError(errors.New("DHT timeout value must be >= 0"), cmdkit.ErrNormal)
					return
				}
				ropts = append(ropts, nsopts.DhtTimeout(d))
			}
			p, err := n.Namesys.Resolve(req.Context, name, ropts...)
			// ErrResolveRecursion is fine
			if err != nil && err != ns.ErrResolveRecursion {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}
			cmds.EmitOnce(res, &ncmd.ResolvedPath{Path: p})
			return
		}

		// else, ipfs path or ipns with recursive flag
		p, err := path.ParsePath(name)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		r := &resolver.Resolver{
			DAG:         n.DAG,
			ResolveOnce: uio.ResolveUnixfsOnce,
		}

		node, err := core.Resolve(req.Context, n.Namesys, r, p)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		c := node.Cid()

		cmds.EmitOnce(res, &ncmd.ResolvedPath{Path: path.FromCid(c)})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeEncoder(func(req *cmds.Request, w io.Writer, v interface{}) error {
			output, ok := v.(*ncmd.ResolvedPath)
			if !ok {
				return e.TypeErr(output, v)
			}

			fmt.Fprintln(w, output.Path.String())
			return nil
		}),
	},
	Type: ncmd.ResolvedPath{},
}
