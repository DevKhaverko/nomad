package structs

import "fmt"

// IngressClass is an enum string that encapsulates the valid options for a
// Ingress Plugin block's Type. These modes will allow the plugin to be used in
// different ways by the client.
type IngressClass string

const (
	// InternalIngressClass indicates that load balancer is inside nomad cluster
	InternalIngressClass IngressClass = "internal"
	// ExternalIngressClass indicates that load balancer is outside nomad cluster
	ExternalIngressClass IngressClass = "external"
	IngresPluginType     string       = "ingress"
	IngressSocketName                 = "ingress.sock"
)

// TaskIngressPluginConfig contains the data that is required to setup a task as a
// Ingress plugin. This will be used by the ingress_plugin_supervisor_hook to initiate the connection to the plugin catalog.
type TaskIngressPluginConfig struct {
	ID            string
	NomadEndpoint string                      `mapstructure:"nomad_endpoint" hcl:"nomad_endpoint"`
	NomadToken    string                      `mapstructure:"nomad_token" hcl:"nomad_token"`
	Class         IngressClass                `mapstructure:"class" hcl:"class"`
	Internal      *InternalIngressClassConfig `mapstructure:"internal" hcl:"internal,optional"`
	External      *ExternalIngressClassConfig `mapstructure:"external" hcl:"external,optional"`
}

// InternalIngressClassConfig contains parameters for ingress controller
// which manages load balancer inside nomad cluster
type InternalIngressClassConfig struct {
	LoadBalancerConfigurationPath string `mapstructure:"lb_conf_path" hcl:"lb_conf_path"`
}

// ExternalIngressClassConfig contains parameters for ingress controller
// which manages load balancer outside nomad cluster
type ExternalIngressClassConfig struct {
}

func (t *TaskIngressPluginConfig) Copy() *TaskIngressPluginConfig {
	if t == nil {
		return nil
	}

	nt := new(TaskIngressPluginConfig)
	*nt = *t
	return nt
}

func (t *TaskIngressPluginConfig) Equal(o *TaskIngressPluginConfig) bool {
	if t == nil || o == nil {
		return t == o
	}
	switch {
	case t.ID != o.ID:
		return false
	case t.NomadEndpoint != o.NomadEndpoint:
		return false
	case t.NomadToken != o.NomadToken:
		return false
	case t.Class != t.Class:
		return false
	}

	return t.Internal.Equal(o.Internal) && t.External.Equal(o.External)
}

func (i *InternalIngressClassConfig) Equal(o *InternalIngressClassConfig) bool {
	if i == nil || o == nil {
		return i == o
	}
	switch {
	case i.LoadBalancerConfigurationPath != o.LoadBalancerConfigurationPath:
		return false
	}
	return true
}

// TODO add checks after making external
func (e *ExternalIngressClassConfig) Equal(o *ExternalIngressClassConfig) bool {
	if e == nil || o == nil {
		return e == o
	}
	return true
}

type IngressPlugin struct {
	ID       string
	Provider string
	Version  string

	Controllers map[string]*IngressInfo

	// Allocations are populated by denormalize to show running allocations
	Allocations []*AllocListStub

	// Jobs are populated to by job update to support expected counts and the UI
	ControllerJobs JobDescriptions

	// Cache the count of healthy plugins
	ControllersHealthy  int
	ControllersExpected int

	CreateIndex uint64
	ModifyIndex uint64
}

func NewIngressPlugin(id string, index uint64) *IngressPlugin {
	out := &IngressPlugin{
		ID:          id,
		CreateIndex: index,
		ModifyIndex: index,
	}

	out.newStructs()
	return out
}

func (i *IngressPlugin) newStructs() {
	i.Controllers = map[string]*IngressInfo{}
	i.ControllerJobs = make(JobDescriptions)
}

func (i *IngressPlugin) Copy() *IngressPlugin {
	copy := *i
	out := &copy
	out.newStructs()

	for k, v := range i.Controllers {
		out.Controllers[k] = v.Copy()
	}

	for k, v := range i.ControllerJobs {
		out.ControllerJobs[k] = v.Copy()
	}

	return out
}

func (i *IngressPlugin) AddPlugin(nodeID string, info *IngressInfo) error {
	prev, ok := i.Controllers[nodeID]
	if ok {
		if prev == nil {
			return fmt.Errorf("plugin missing node: %s", nodeID)
		}
		if prev.Healthy {
			i.ControllersHealthy -= 1
		}
	}
	if prev != nil || info.Healthy {
		i.Controllers[nodeID] = info
	}
	if info.Healthy {
		i.ControllersHealthy += 1
	}
	return nil
}

func (i *IngressPlugin) IsEmpty() bool {
	return i == nil ||
		len(i.Controllers) == 0 &&
			i.ControllerJobs.Count() == 0
}

func (i *IngressPlugin) DeleteNodeForType(nodeID string) error {
	if prev, ok := i.Controllers[nodeID]; ok {
		if prev == nil {
			return fmt.Errorf("plugin missing controller: %s", nodeID)
		}
		if prev.Healthy {
			i.ControllersHealthy--
		}
		delete(i.Controllers, nodeID)
	}
	return nil
}

func (i *IngressPlugin) DeleteAlloc(allocID, nodeID string) error {
	prev, ok := i.Controllers[nodeID]
	if ok {
		if prev == nil {
			return fmt.Errorf("plugin missing controller: %s", nodeID)
		}
		if prev.AllocID == allocID {
			if prev.Healthy {
				i.ControllersHealthy -= 1
			}
			delete(i.Controllers, nodeID)
		}
	}

	return nil
}

func (i *IngressPlugin) AddJob(job *Job, summary *JobSummary) {
	i.UpdateExpectedWithJob(job, summary, false)
}

func (i *IngressPlugin) DeleteJob(job *Job, summary *JobSummary) {
	i.UpdateExpectedWithJob(job, summary, true)
}

func (i *IngressPlugin) UpdateExpectedWithJob(job *Job, summary *JobSummary, terminal bool) {
	var count int

	for _, tg := range job.TaskGroups {
		if job.Type == JobTypeSystem {
			if summary == nil {
				continue
			}

			s, ok := summary.Summary[tg.Name]
			if !ok {
				continue
			}

			count = s.Running + s.Queued + s.Starting
		} else {
			count = tg.Count
		}

		for _, t := range tg.Tasks {
			if t.IngressPluginConfig == nil ||
				t.IngressPluginConfig.ID != i.ID {
				continue
			}

			if terminal {
				i.ControllerJobs.Delete(job)
			} else {
				i.ControllerJobs.Add(job, count)
			}
		}
	}

	i.ControllersExpected = i.ControllerJobs.Count()
}

type IngressPluginDeleteRequest struct {
	ID string
	QueryOptions
}

type IngressPluginDeleteResponse struct {
	QueryMeta
}

type IngressPluginListRequest struct {
	QueryOptions
}

type IngressPluginListStub struct {
	ID                  string
	Provider            string
	ControllersHealthy  int
	ControllersExpected int
	CreateIndex         uint64
	ModifyIndex         uint64
}

func (i *IngressPlugin) Stub() *IngressPluginListStub {
	return &IngressPluginListStub{
		ID:                  i.ID,
		Provider:            i.Provider,
		ControllersHealthy:  i.ControllersHealthy,
		ControllersExpected: i.ControllersExpected,
		CreateIndex:         i.CreateIndex,
		ModifyIndex:         i.ModifyIndex,
	}
}

type IngressPluginListResponse struct {
	Plugins []*IngressPluginListStub
	QueryMeta
}
