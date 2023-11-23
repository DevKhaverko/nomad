package taskrunner

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	ti "github.com/hashicorp/nomad/client/allocrunner/taskrunner/interfaces"
	"github.com/hashicorp/nomad/nomad/structs"
	"os"
	"sync"
)

// ingressPluginSupervisorHook manages supervising plugins that are running as Nomad
// tasks. These plugins will be fingerprinted, and it will manage connecting them
// to their requisite plugin manager.
type ingressPluginSupervisorHook struct {
	logger hclog.Logger
	alloc  *structs.Allocation
	task   *structs.Task
	runner *TaskRunner

	lbConfPath string
	// eventEmitter is used to emit events to the task
	eventEmitter ti.EventEmitter
	lifecycle    ti.TaskLifecycle

	shutdownCtx      context.Context
	shutdownCancelFn context.CancelFunc
	runOnce          sync.Once

	// previousHealthstate is used by the supervisor goroutine to track historic
	// health states for gating task events.
	previousHealthState bool
}

type ingressPluginSupervisorHookConfig struct {
	events     ti.EventEmitter
	runner     *TaskRunner
	lifecycle  ti.TaskLifecycle
	lbConfPath string
	logger     hclog.Logger
}

func newIngressPluginSupervisorHook(c *ingressPluginSupervisorHookConfig) *ingressPluginSupervisorHook {
	task := c.runner.Task()

	shutdownCtx, cancelFn := context.WithCancel(context.Background())

	hook := &ingressPluginSupervisorHook{
		alloc:            c.runner.Alloc(),
		runner:           c.runner,
		lifecycle:        c.lifecycle,
		logger:           c.logger,
		task:             task,
		lbConfPath:       c.lbConfPath,
		shutdownCtx:      shutdownCtx,
		shutdownCancelFn: cancelFn,
		eventEmitter:     c.events,
	}

	return hook
}

func (*ingressPluginSupervisorHook) Name() string {
	return "ingress_plugin_supervisor"
}

// Prestart is called before the task is started including after every
// restart. This requires that the mount paths for a plugin be
// idempotent, despite us not knowing the name of the plugin ahead of
// time.  Because of this, we use the allocid_taskname as the unique
// identifier for a plugin on the filesystem.
func (i *ingressPluginSupervisorHook) Prestart(
	ctx context.Context,
	req *interfaces.TaskPrestartRequest,
	resp *interfaces.TaskPrestartResponse) error {
	if i.lbConfPath != "" {
		if err := os.MkdirAll(i.lbConfPath, 700); err != nil && !os.IsExist(err) {
			return fmt.Errorf("failed to create dir for lb conf")
		}
	}

	i.setLBConfHook()
	return nil
}

func (i *ingressPluginSupervisorHook) setLBConfHook() {
	pluginInfo, err := i.runner.dynamicRegistry.PluginForAlloc(
		string(i.task.IngressPluginConfig.Class),
		i.task.IngressPluginConfig.ID,
		i.alloc.ID,
	)
	if err != nil {
		i.logger.Error("", err)
		os.Exit(1)
	}
	if pluginInfo != nil && pluginInfo.Options["lb_conf_path"] != "" {
		i.lbConfPath = pluginInfo.Options["lb_conf_path"]
	}
}

// Poststart is called after the task has started. Poststart is not
// called if the allocation is terminal.
//
// The context is cancelled if the task is killed.
func (i *ingressPluginSupervisorHook) Poststart(_ context.Context,
	_ *interfaces.TaskPoststartRequest,
	_ *interfaces.TaskPoststartResponse) error {

	// If we're already running the supervisor routine, then we don't need to try
	// and restart it here as it only terminates on `Stop` hooks.
	i.runOnce.Do(func() {
		i.setLBConfHook()
		go i.ensureSupervisorLoop(i.shutdownCtx)
	})
	return nil
}

// TODO add logic of checking health
func (i *ingressPluginSupervisorHook) ensureSupervisorLoop(ctx context.Context) {

}

// TODO add logic check health once
func (i *ingressPluginSupervisorHook) supervisorLoopOnce(ctx context.Context) (bool, error) {

	return
}