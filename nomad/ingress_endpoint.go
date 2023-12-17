package nomad

import (
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

const ingressPluginTable = "ingress_plugin"

type IngressPlugin struct {
	srv    *Server
	ctx    *RPCContext
	logger hclog.Logger
}

func NewIngressPluginEndpoint(srv *Server, ctx *RPCContext) *IngressPlugin {
	return &IngressPlugin{srv: srv, ctx: ctx, logger: srv.logger.Named("ingress_plugin")}
}

func (i *IngressPlugin) List(args *structs.IngressPluginListRequest, reply *structs.IngressPluginListResponse) error {

	if done, err := i.srv.forward("IngressPlugin.List", args, args, reply); done {
		return err
	}

	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {

			var iter memdb.ResultIterator
			var err error
			// Query all plugins
			iter, err = state.IngressPlugins(ws)
			if err != nil {
				return err
			}

			// Collect results
			ps := []*structs.IngressPluginListStub{}
			for {
				raw := iter.Next()
				if raw == nil {
					break
				}

				plug := raw.(*structs.IngressPlugin)
				ps = append(ps, plug.Stub())
			}

			reply.Plugins = ps
			return i.srv.replySetIndex(ingressPluginTable, &reply.QueryMeta)
		}}
	return i.srv.blockingRPC(&opts)
}

func (v *IngressPlugin) Get(args *structs.IngressPluginGetRequest, reply *structs.IngressPluginGetResponse) error {

	if done, err := v.srv.forward("IngressPlugin.Get", args, args, reply); done {
		return err
	}

	if args.ID == "" {
		return fmt.Errorf("missing plugin ID")
	}

	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			snap, err := state.Snapshot()
			if err != nil {
				return err
			}

			plug, err := snap.IngressPluginByID(ws, args.ID)
			if err != nil {
				return err
			}

			if plug == nil {
				return nil
			}

			plug, err = snap.IngressPluginDenormalize(ws, plug.Copy())
			if err != nil {
				return err
			}

			// Filter the allocation stubs by our namespace. withAllocs
			// means we're allowed
			var as []*structs.AllocListStub
			for _, a := range plug.Allocations {
				if a.Namespace == args.RequestNamespace() {
					as = append(as, a)
				}
			}
			plug.Allocations = as

			reply.Plugin = plug
			return v.srv.replySetIndex(ingressPluginTable, &reply.QueryMeta)
		}}
	return v.srv.blockingRPC(&opts)
}
