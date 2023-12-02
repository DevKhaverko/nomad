package structs

// IngressClass is an enum string that encapsulates the valid options for a
// Ingress Plugin block's Type. These modes will allow the plugin to be used in
// different ways by the client.
type IngressClass string

const (
	// InternalIngressClass indicates that load balancer is inside nomad cluster
	InternalIngressClass IngressClass = "internal"
	// ExternalIngressClass indicates that load balancer is outside nomad cluster
	ExternalIngressClass IngressClass = "external"
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
