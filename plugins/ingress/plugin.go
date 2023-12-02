package ingress

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper/grpc-middleware/logging"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"google.golang.org/grpc"
	"net"
	"os"
	"time"
)

const IngressPluginType = "ingress"

type IngressPlugin interface {
	base.BasePlugin
	PluginProbe(ctx context.Context) (bool, error)
	CreateRoutes(allocID string) error
	ChangeOrDeleteRoutes(allocID string) error
	PluginGetInfo(ctx context.Context) (string, string, error)
	Close() error
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

func (c *client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *client) ensureConnected(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("client not initialized")
	}
	if c.conn != nil {
		return nil
	}
	if c.addr == "" {
		return fmt.Errorf("address is empty")
	}
	var conn *grpc.ClientConn
	var err error
	t := time.NewTimer(0)
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout while connecting to gRPC socket: %v", err)
		case <-t.C:
			_, err = os.Stat(c.addr)
			if err != nil {
				err = fmt.Errorf("failed to stat socket: %v", err)
				t.Reset(5 * time.Second)
				continue
			}
			conn, err = newGrpcConn(c.addr, c.logger)
			if err != nil {
				err = fmt.Errorf("failed to create gRPC connection: %v", err)
				t.Reset(time.Second * 5)
				continue
			}
			c.conn = conn
			return nil
		}
	}
}

func newGrpcConn(addr string, logger hclog.Logger) (*grpc.ClientConn, error) {
	// after DialContext returns w/ initial connection, closing this
	// context is a no-op
	connectCtx, cancel := context.WithTimeout(context.Background(), time.Second*1)
	defer cancel()
	conn, err := grpc.DialContext(
		connectCtx,
		addr,
		grpc.WithBlock(),
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(logging.UnaryClientInterceptor(logger)),
		grpc.WithStreamInterceptor(logging.StreamClientInterceptor(logger)),
		grpc.WithAuthority("localhost"),
		grpc.WithDialer(func(target string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", target, timeout)
		}),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to open grpc connection to addr: %s, err: %v", addr, err)
	}

	return conn, nil
}

func (c *client) PluginProbe(ctx context.Context) (bool, error) {
	return true, nil
}

func (c *client) PluginInfo() (*base.PluginInfoResponse, error) {
	//ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	//defer cancel()
	//if err := c.ensureConnected(ctx); err != nil {
	//	return nil, err
	//}

	// note: no grpc retries needed here, as this is called in
	// fingerprinting and will get retried by the caller.
	name := "test"
	version := "1.0.0"
	var err error = nil
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

func (c *client) PluginGetInfo(ctx context.Context) (string, string, error) {
	return "", "", nil
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
