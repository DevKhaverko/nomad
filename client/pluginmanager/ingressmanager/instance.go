package ingressmanager

import (
	"context"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/dynamicplugins"
	"github.com/hashicorp/nomad/plugins/ingress"
	"time"
)

const managerFingerprintInterval = 3 * time.Second

// instanceManager is used to manage the fingerprinting and supervision of a
// single CSI Plugin.
type instanceManager struct {
	info    *dynamicplugins.PluginInfo
	logger  hclog.Logger
	eventer TriggerNodeEvent

	updater UpdateIngressInfoFunc

	shutdownCtx         context.Context
	shutdownCtxCancelFn context.CancelFunc
	shutdownCh          chan struct{}

	// AllocID is the allocation id of the task group running the dynamic plugin
	allocID string

	fp *pluginFingerprinter

	client ingress.IngressPlugin
}

func newInstanceManager(logger hclog.Logger, eventer TriggerNodeEvent, updater UpdateIngressInfoFunc, p *dynamicplugins.PluginInfo) *instanceManager {
	ctx, cancelFn := context.WithCancel(context.Background())
	logger = logger.Named(p.Name)
	return &instanceManager{
		logger:  logger,
		eventer: eventer,
		info:    p,
		updater: updater,

		fp: &pluginFingerprinter{
			logger:                          logger.Named("fingerprinter"),
			info:                            p,
			hadFirstSuccessfulFingerprintCh: make(chan struct{}),
		},

		allocID: p.AllocID,

		shutdownCtx:         ctx,
		shutdownCtxCancelFn: cancelFn,
		shutdownCh:          make(chan struct{}),
	}
}

func (i *instanceManager) run() {
	c := ingress.NewClient(i.info.ConnectionInfo.SocketPath, i.logger)
	i.client = c
	i.fp.client = c

	go i.runLoop()
}

func (i *instanceManager) requestCtxWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(i.shutdownCtx, timeout)
}

func (i *instanceManager) runLoop() {
	timer := time.NewTimer(0)
	for {
		select {
		case <-i.shutdownCtx.Done():
			if i.client != nil {
				i.client.Close()
				i.client = nil
			}

			// run one last fingerprint so that we mark the plugin as unhealthy.
			// the client has been closed so this will return quickly with the
			// plugin's basic info
			ctx, cancelFn := i.requestCtxWithTimeout(time.Second)
			info := i.fp.fingerprint(ctx)
			cancelFn()
			if info != nil {
				i.updater(i.info.Name, info)
			}
			close(i.shutdownCh)
			return

		case <-timer.C:
			ctx, cancelFn := i.requestCtxWithTimeout(managerFingerprintInterval)
			info := i.fp.fingerprint(ctx)
			cancelFn()
			if info != nil {
				i.updater(i.info.Name, info)
			}
			timer.Reset(managerFingerprintInterval)
		}
	}
}

func (i *instanceManager) shutdown() {
	i.shutdownCtxCancelFn()
	<-i.shutdownCh
}
