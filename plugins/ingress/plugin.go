package ingress

import (
	"context"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"google.golang.org/grpc"
	"time"
)

const IngressPluginType = "ingress"

type IngressPlugin interface {
	base.BasePlugin
	CreateRoutes(allocID string) error
	ChangeOrDeleteRoutes(allocID string) error
	PluginGetInfo(ctx context.Context) (string, string, error)
}

type client struct {
	addr   string
	conn   *grpc.ClientConn
	logger hclog.Logger
}

func NewClient(addr string, logger hclog.Logger) IngressPlugin {
	return &client{
		addr:   addr,
		logger: logger,
	}
}

func (c *client) PluginInfo() (*base.PluginInfoResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := c.ensureConnected(ctx); err != nil {
		return nil, err
	}

	// note: no grpc retries needed here, as this is called in
	// fingerprinting and will get retried by the caller.
	name, version, err := c.PluginGetInfo(ctx)
	if err != nil {
		return nil, err
	}

	return &base.PluginInfoResponse{
		Type:              IngressPluginType, // note: this isn't a Nomad go-plugin type
		PluginApiVersions: []string{"1.0.0"}, // TODO(tgross): we want to fingerprint spec version, but this isn't included as a field from the plugins
		PluginVersion:     version,
		Name:              name,
	}, nil
}

func (c *client) ensureConnected(ctx context.Context) error {

}

func (c *client) PluginGetInfo(ctx context.Context) (string, string, error) {

}

func (c *client) ConfigSchema() (*hclspec.Spec, error) {
	//TODO implement me
	panic("implement me")
}

func (c *client) SetConfig(config *base.Config) error {
	//TODO implement me
	panic("implement me")
}

func (c *client) CreateRoutes(allocID string) error {
	//TODO implement me
	panic("implement me")
}

func (c *client) ChangeOrDeleteRoutes(allocID string) error {
	//TODO implement me
	panic("implement me")
}
