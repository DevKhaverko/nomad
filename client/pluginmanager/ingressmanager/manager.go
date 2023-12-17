package ingressmanager

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/dynamicplugins"
	"github.com/hashicorp/nomad/client/pluginmanager"
	"github.com/hashicorp/nomad/nomad/structs"
	"sync"
	"time"
)

type Manager interface {
	// PluginManager returns a PluginManager for use by the node fingerprinter.
	PluginManager() pluginmanager.PluginManager

	// WaitForPlugin waits for the plugin to become available,
	// or until its context is canceled or times out.
	WaitForPlugin(ctx context.Context, pluginType, pluginID string) error

	// Shutdown shuts down the Manager and unmounts any locally attached volumes.
	Shutdown()
}

// defaultPluginResyncPeriod is the time interval used to do a full resync
// against the dynamicplugins, to account for missed updates.
const defaultPluginResyncPeriod = 30 * time.Second

// UpdateIngressInfoFunc is the callback used to update the node from
// fingerprinting
type UpdateIngressInfoFunc func(string, *structs.IngressInfo)
type TriggerNodeEvent func(*structs.NodeEvent)

type Config struct {
	Logger                hclog.Logger
	DynamicRegistry       dynamicplugins.Registry
	UpdateIngressInfoFunc UpdateIngressInfoFunc
	PluginResyncPeriod    time.Duration
	TriggerNodeEvent      TriggerNodeEvent
}

// New returns a new PluginManager that will handle managing Ingress plugins from
// the dynamicRegistry from the provided Config.
func New(config *Config) Manager {
	// Use a dedicated internal context for managing plugin shutdown.
	ctx, cancelFn := context.WithCancel(context.Background())
	if config.PluginResyncPeriod == 0 {
		config.PluginResyncPeriod = defaultPluginResyncPeriod
	}

	return &ingressManager{
		logger:    config.Logger.Named("ingress_manager"),
		eventer:   config.TriggerNodeEvent,
		registry:  config.DynamicRegistry,
		instances: make(map[string]map[string]*instanceManager),

		updateIngressInfoFunc: config.UpdateIngressInfoFunc,
		pluginResyncPeriod:    config.PluginResyncPeriod,

		shutdownCtx:         ctx,
		shutdownCtxCancelFn: cancelFn,
		shutdownCh:          make(chan struct{}),
	}
}

type ingressManager struct {
	// instances should only be accessed after locking with instancesLock.
	// It is a map of PluginType : [PluginName : *instanceManager]
	instances     map[string]map[string]*instanceManager
	instancesLock sync.RWMutex

	registry           dynamicplugins.Registry
	logger             hclog.Logger
	eventer            TriggerNodeEvent
	pluginResyncPeriod time.Duration

	updateIngressInfoFunc UpdateIngressInfoFunc

	shutdownCtx         context.Context
	shutdownCtxCancelFn context.CancelFunc
	shutdownCh          chan struct{}
}

func (c *ingressManager) PluginManager() pluginmanager.PluginManager {
	return c
}

// WaitForPlugin waits for a specific plugin to be registered and available,
// unless the context is canceled, or it takes longer than a minute.
func (c *ingressManager) WaitForPlugin(ctx context.Context, pType, pID string) error {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	p, err := c.registry.WaitForPlugin(ctx, pType, pID)
	if err != nil {
		return fmt.Errorf("%s plugin '%s' did not become ready: %w", pType, pID, err)
	}
	c.instancesLock.Lock()
	defer c.instancesLock.Unlock()
	c.ensureInstance(p)
	return nil
}

// Run starts a plugin manager and should return early
func (c *ingressManager) Run() {
	go c.runLoop()
}

func (c *ingressManager) runLoop() {
	timer := time.NewTimer(0) // ensure we sync immediately in first pass
	controllerUpdates := c.registry.PluginsUpdatedCh(c.shutdownCtx, "ingress")
	for {
		select {
		case <-timer.C:
			c.resyncPluginsFromRegistry("ingress")
			timer.Reset(c.pluginResyncPeriod)
		case event := <-controllerUpdates:
			c.handlePluginEvent(event)
		case <-c.shutdownCtx.Done():
			close(c.shutdownCh)
			return
		}
	}
}

// resyncPluginsFromRegistry does a full sync of the running instance
// managers against those in the registry. we primarily will use update
// events from the registry.
func (c *ingressManager) resyncPluginsFromRegistry(ptype string) {

	c.instancesLock.Lock()
	defer c.instancesLock.Unlock()

	plugins := c.registry.ListPlugins(ptype)
	seen := make(map[string]struct{}, len(plugins))

	// For every plugin in the registry, ensure that we have an existing plugin
	// running. Also build the map of valid plugin names.
	// Note: monolith plugins that run as both controllers and nodes get a
	// separate instance manager for both modes.
	for _, plugin := range plugins {
		seen[plugin.Name] = struct{}{}
		c.ensureInstance(plugin)
	}

	// For every instance manager, if we did not find it during the plugin
	// iterator, shut it down and remove it from the table.
	instances := c.instancesForType(ptype)
	for name, mgr := range instances {
		if _, ok := seen[name]; !ok {
			c.ensureNoInstance(mgr.info)
		}
	}
}

// handlePluginEvent syncs a single event against the plugin registry
func (c *ingressManager) handlePluginEvent(event *dynamicplugins.PluginUpdateEvent) {
	if event == nil || event.Info == nil {
		return
	}
	c.logger.Trace("dynamic plugin event",
		"event", event.EventType,
		"plugin_id", event.Info.Name,
		"plugin_alloc_id", event.Info.AllocID)

	c.instancesLock.Lock()
	defer c.instancesLock.Unlock()

	switch event.EventType {
	case dynamicplugins.EventTypeRegistered:
		c.ensureInstance(event.Info)
	case dynamicplugins.EventTypeDeregistered:
		c.ensureNoInstance(event.Info)
	default:
		c.logger.Error("received unknown dynamic plugin event type",
			"type", event.EventType)
	}
}

// Ensure we have an instance manager for the plugin and add it to
// the CSI manager's tracking table for that plugin type.
// Assumes that c.instances has been locked.
func (c *ingressManager) ensureInstance(plugin *dynamicplugins.PluginInfo) {
	name := plugin.Name
	ptype := plugin.Type
	instances := c.instancesForType(ptype)
	mgr, ok := instances[name]
	if !ok {
		c.logger.Debug("detected new ingress plugin", "name", name, "type", ptype, "alloc", plugin.AllocID)
		mgr := newInstanceManager(c.logger, c.eventer, c.updateIngressInfoFunc, plugin)
		instances[name] = mgr
		mgr.run()
	} else if mgr.allocID != plugin.AllocID {
		mgr.shutdown()
		c.logger.Debug("detected update for ingress plugin", "name", name, "type", ptype, "alloc", plugin.AllocID)
		mgr := newInstanceManager(c.logger, c.eventer, c.updateIngressInfoFunc, plugin)
		instances[name] = mgr
		mgr.run()

	}
}

func (c *ingressManager) ensureNoInstance(plugin *dynamicplugins.PluginInfo) {
	name := plugin.Name
	ptype := plugin.Type
	instances := c.instancesForType(ptype)
	if mgr, ok := instances[name]; ok {
		if mgr.allocID == plugin.AllocID {
			c.logger.Debug("shutting down ingress plugin", "name", name, "type", ptype, "alloc", plugin.AllocID)
			mgr.shutdown()
			delete(instances, name)
		}
	}
}

// Get the instance managers table for a specific plugin type,
// ensuring it's been initialized if it doesn't exist.
// Assumes that c.instances has been locked.
func (c *ingressManager) instancesForType(ptype string) map[string]*instanceManager {
	pluginMap, ok := c.instances[ptype]
	if !ok {
		pluginMap = make(map[string]*instanceManager)
		c.instances[ptype] = pluginMap
	}
	return pluginMap
}

// Shutdown should gracefully shutdown all plugins managed by the manager.
// It must block until shutdown is complete
func (c *ingressManager) Shutdown() {
	// Shut down the run loop
	c.shutdownCtxCancelFn()

	// Wait for plugin manager shutdown to complete so that we
	// don't try to shutdown instance managers while runLoop is
	// doing a resync
	<-c.shutdownCh

	// Shutdown all the instance managers in parallel
	var wg sync.WaitGroup
	for _, pluginMap := range c.instances {
		for _, mgr := range pluginMap {
			wg.Add(1)
			go func(mgr *instanceManager) {
				mgr.shutdown()
				wg.Done()
			}(mgr)
		}
	}
	wg.Wait()
}

// PluginType is the type of plugin which the manager manages
func (c *ingressManager) PluginType() string {
	return "ingress"
}
