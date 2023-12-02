package taskrunner

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	ti "github.com/hashicorp/nomad/client/allocrunner/taskrunner/interfaces"
	"github.com/hashicorp/nomad/client/dynamicplugins"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/ingress"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ingressPluginSupervisorHook manages supervising plugins that are running as Nomad
// tasks. These plugins will be fingerprinted, and it will manage connecting them
// to their requisite plugin manager.
type ingressPluginSupervisorHook struct {
	logger           hclog.Logger
	alloc            *structs.Allocation
	task             *structs.Task
	runner           *TaskRunner
	socketMountPoint string
	socketPath       string
	lbConfPath       string
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
	clientStateDirPath string
	events             ti.EventEmitter
	runner             *TaskRunner
	lifecycle          ti.TaskLifecycle
	lbConfPath         string
	logger             hclog.Logger
}

func newIngressPluginSupervisorHook(c *ingressPluginSupervisorHookConfig) *ingressPluginSupervisorHook {
	socketMountPoint := filepath.Join(c.clientStateDirPath, "ingress",
		"plugins", c.runner.Alloc().ID)

	task := c.runner.Task()

	shutdownCtx, cancelFn := context.WithCancel(context.Background())

	hook := &ingressPluginSupervisorHook{
		alloc:            c.runner.Alloc(),
		runner:           c.runner,
		socketMountPoint: socketMountPoint,
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

	if err := os.MkdirAll(i.socketMountPoint, 0700); err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to create socket mount point: %v", err)
	}

	// where the socket will be mounted
	configMount := &drivers.MountConfig{
		TaskPath:        "/opt",
		HostPath:        i.socketMountPoint,
		Readonly:        false,
		PropagationMode: "bidirectional",
	}

	i.setSocketHook()

	if _, ok := i.task.Env["INGRESS_ENDPOINT"]; !ok {
		resp.Env = map[string]string{
			"CSI_ENDPOINT": "unix://" + i.socketPath}
	}

	mounts := ensureMountpointInserted(i.runner.hookResources.getMounts(), configMount)
	i.runner.hookResources.setMounts(mounts)
	return nil
}

func (i *ingressPluginSupervisorHook) setSocketHook() {
	pluginInfo, err := i.runner.dynamicRegistry.PluginForAlloc(
		string(i.task.IngressPluginConfig.Class),
		i.task.IngressPluginConfig.ID,
		i.alloc.ID,
	)
	if err != nil {
		i.logger.Error("", err)
		os.Exit(1)
	}
	if pluginInfo != nil && pluginInfo.ConnectionInfo.SocketPath != "" {
		i.socketPath = pluginInfo.ConnectionInfo.SocketPath
		return
	}
	i.socketPath = filepath.Join(i.socketMountPoint, structs.IngressSocketName)
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
		i.setSocketHook()
		go i.ensureSupervisorLoop(i.shutdownCtx)
	})
	return nil
}

func (i *ingressPluginSupervisorHook) ensureSupervisorLoop(ctx context.Context) {
	client := ingress.NewClient(i.socketPath, i.logger.Named("ingress_client").With(
		"plugin.name", i.task.IngressPluginConfig.ID))
	defer client.Close()

	t := time.NewTimer(0)

	// We're in Poststart at this point, so if we can't connect within
	// this deadline, assume it's broken so we can restart the task
	startCtx, startCancelFn := context.WithTimeout(ctx, 30*time.Second)
	defer startCancelFn()

	var err error
	var pluginHealthy bool

WAITFORREADY:
	for {
		select {
		case <-startCtx.Done():
			i.kill(ctx, fmt.Errorf("Ingress plugin failed probe: %v", err))
			return
		case <-t.C:
			pluginHealthy, err = i.supervisorLoopOnce(startCtx, client)
			if err != nil || !pluginHealthy {
				i.logger.Debug("Ingress plugin not ready", "error", err)
				// Use only a short delay here to optimize for quickly
				// bringing up a plugin
				t.Reset(5 * time.Second)
				continue
			}
			// Mark the plugin as healthy in a task event
			i.logger.Debug("Ingress plugin is ready")
			i.previousHealthState = pluginHealthy
			event := structs.NewTaskEvent(structs.TaskPluginHealthy)
			event.SetMessage(fmt.Sprintf("plugin: %s", i.task.IngressPluginConfig.ID))
			i.eventEmitter.EmitEvent(event)

			break WAITFORREADY
		}
	}

	// Step 2: Register the plugin with the catalog.
	deregisterPluginFn, err := i.registerPlugin(client, i.socketPath)
	if err != nil {
		i.kill(ctx, fmt.Errorf("CSI plugin failed to register: %v", err))
		return
	}
	// De-register plugins on task shutdown
	defer deregisterPluginFn()

	// Step 3: Start the lightweight supervisor loop. At this point,
	// probe failures don't cause the task to restart
	t.Reset(0)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			pluginHealthy, err := i.supervisorLoopOnce(ctx, client)
			if err != nil {
				i.logger.Error("Ingress plugin fingerprinting failed", "error", err)
			}

			// The plugin has transitioned to a healthy state. Emit an event.
			if !i.previousHealthState && pluginHealthy {
				event := structs.NewTaskEvent(structs.TaskPluginHealthy)
				event.SetMessage(fmt.Sprintf("plugin: %s", i.task.IngressPluginConfig.ID))
				i.eventEmitter.EmitEvent(event)
			}

			// The plugin has transitioned to an unhealthy state. Emit an event.
			if i.previousHealthState && !pluginHealthy {
				event := structs.NewTaskEvent(structs.TaskPluginUnhealthy)
				if err != nil {
					event.SetMessage(fmt.Sprintf("Error: %v", err))
				} else {
					event.SetMessage("Unknown Reason")
				}
				i.eventEmitter.EmitEvent(event)
			}

			i.previousHealthState = pluginHealthy

			// This loop is informational and in some plugins this may be expensive to
			// validate. We use a longer timeout (30s) to avoid causing undue work.
			t.Reset(30 * time.Second)
		}
	}
}

func (i *ingressPluginSupervisorHook) registerPlugin(client ingress.IngressPlugin, socketPath string) (func(), error) {
	// At this point we know the plugin is ready and we can fingerprint it
	// to get its vendor name and version
	info, err := client.PluginInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to probe plugin: %v", err)
	}

	mkInfoFn := func(pluginType string) *dynamicplugins.PluginInfo {
		return &dynamicplugins.PluginInfo{
			Type:    pluginType,
			Name:    i.task.IngressPluginConfig.ID,
			Version: info.PluginVersion,
			ConnectionInfo: &dynamicplugins.PluginConnectionInfo{
				SocketPath: socketPath,
			},
			AllocID: i.alloc.ID,
			Options: map[string]string{
				"Provider": info.Name, // vendor name
			},
		}
	}

	registration := mkInfoFn(dynamicplugins.PluginTypeIngress)

	deregistrationFn := func() {}

	if err := i.runner.dynamicRegistry.RegisterPlugin(registration); err != nil {
		deregistrationFn()
		return nil, err
	}

	// need to rebind these so that each deregistration function
	// closes over its own registration
	rname := registration.Name
	rtype := registration.Type
	allocID := registration.AllocID
	deregistrationFn = func() {
		err := i.runner.dynamicRegistry.DeregisterPlugin(rtype, rname, allocID)
		if err != nil {
			i.logger.Error("failed to deregister ingress plugin", "name", rname, "type", rtype, "error", err)
		}
	}
	return func() {
		deregistrationFn()
	}, nil
}

// TODO add logic check health once
func (i *ingressPluginSupervisorHook) supervisorLoopOnce(ctx context.Context, client ingress.IngressPlugin) (bool, error) {
	probeCtx, probeCancelFn := context.WithTimeout(ctx, 5*time.Second)
	defer probeCancelFn()

	healthy, err := client.PluginProbe(probeCtx)
	if err != nil {
		return false, err
	}

	return healthy, nil
}

// Stop is called after the task has exited and will not be started
// again. It is the only hook guaranteed to be executed whenever
// TaskRunner.Run is called (and not gracefully shutting down).
// Therefore it may be called even when prestart and the other hooks
// have not.
//
// Stop hooks must be idempotent. The context is cancelled prematurely if the
// task is killed.
func (i *ingressPluginSupervisorHook) Stop(_ context.Context, req *interfaces.TaskStopRequest, _ *interfaces.TaskStopResponse) error {
	err := os.RemoveAll(i.socketMountPoint)
	if err != nil {
		i.logger.Error("could not remove plugin socket directory", "dir", i.socketMountPoint, "error", err)
	}
	i.shutdownCancelFn()
	return nil
}

func (i *ingressPluginSupervisorHook) kill(ctx context.Context, reason error) {
	i.logger.Error("killing task because plugin failed", "error", reason)
	event := structs.NewTaskEvent(structs.TaskPluginUnhealthy)
	event.SetMessage(fmt.Sprintf("Error: %v", reason.Error()))
	i.eventEmitter.EmitEvent(event)

	if err := i.lifecycle.Kill(ctx,
		structs.NewTaskEvent(structs.TaskKilling).
			SetFailsTask().
			SetDisplayMessage(fmt.Sprintf("Ingress plugin did not become healthy before configured %v health timeout", "30s")),
	); err != nil {
		i.logger.Error("failed to kill task", "kill_reason", reason, "error", err)
	}
}
