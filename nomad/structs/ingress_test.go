package structs

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestExternalIngressClassConfig_Equal(t *testing.T) {
	type args struct {
		o *ExternalIngressClassConfig
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &ExternalIngressClassConfig{}
			assert.Equalf(t, tt.want, e.Equal(tt.args.o), "Equal(%v)", tt.args.o)
		})
	}
}

func TestInternalIngressClassConfig_Equal(t *testing.T) {
	type fields struct {
		LoadBalancerConfigurationPath string
	}
	type args struct {
		o *InternalIngressClassConfig
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "test o == nil",
			fields: fields{
				LoadBalancerConfigurationPath: "/opt",
			},
			args: args{
				o: nil,
			},
			want: false,
		},
		{
			name: "basic test",
			fields: fields{
				LoadBalancerConfigurationPath: "/opt",
			},
			args: args{
				o: &InternalIngressClassConfig{
					LoadBalancerConfigurationPath: "/opt",
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &InternalIngressClassConfig{
				LoadBalancerConfigurationPath: tt.fields.LoadBalancerConfigurationPath,
			}
			assert.Equalf(t, tt.want, i.Equal(tt.args.o), "Equal(%v)", tt.args.o)
		})
	}
}

func TestTaskIngressPluginConfig_Copy(t1 *testing.T) {
	type fields struct {
		ID            string
		NomadEndpoint string
		NomadToken    string
		Internal      *InternalIngressClassConfig
		External      *ExternalIngressClassConfig
	}
	tests := []struct {
		name   string
		fields fields
		want   *TaskIngressPluginConfig
	}{
		{
			name: "basic",
			fields: fields{
				ID:            "123",
				NomadEndpoint: "my-endpoint",
				NomadToken:    "my-token",
				Internal: &InternalIngressClassConfig{
					LoadBalancerConfigurationPath: "/opt",
				},
				External: nil,
			},
			want: &TaskIngressPluginConfig{
				ID:            "123",
				NomadEndpoint: "my-endpoint",
				NomadToken:    "my-token",
				Internal: &InternalIngressClassConfig{
					LoadBalancerConfigurationPath: "/opt",
				},
				External: nil,
			},
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &TaskIngressPluginConfig{
				ID:            tt.fields.ID,
				NomadEndpoint: tt.fields.NomadEndpoint,
				NomadToken:    tt.fields.NomadToken,
				Internal:      tt.fields.Internal,
				External:      tt.fields.External,
			}
			assert.Equalf(t1, tt.want, t.Copy(), "Copy()")
		})
	}
}

func TestTaskIngressPluginConfig_Equal(t1 *testing.T) {
	type fields struct {
		ID            string
		NomadEndpoint string
		NomadToken    string
		Internal      *InternalIngressClassConfig
		External      *ExternalIngressClassConfig
	}
	type args struct {
		o *TaskIngressPluginConfig
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "test o == nil",
			fields: fields{
				ID:            "123",
				NomadEndpoint: "my-endpoint",
				NomadToken:    "my-token",
				Internal: &InternalIngressClassConfig{
					LoadBalancerConfigurationPath: "/opt",
				},
				External: nil,
			},
			args: args{
				o: nil,
			},
			want: false,
		},
		{
			name: "test equal structs",
			fields: fields{
				ID:            "123",
				NomadEndpoint: "my-endpoint",
				NomadToken:    "my-token",
				Internal: &InternalIngressClassConfig{
					LoadBalancerConfigurationPath: "/opt",
				},
				External: nil,
			},
			args: args{
				o: &TaskIngressPluginConfig{
					ID:            "123",
					NomadEndpoint: "my-endpoint",
					NomadToken:    "my-token",
					Internal: &InternalIngressClassConfig{
						LoadBalancerConfigurationPath: "/opt",
					},
					External: nil,
				},
			},
			want: true,
		},
		{
			name: "test not equal fields",
			fields: fields{
				ID:            "123",
				NomadEndpoint: "my-endpoint",
				NomadToken:    "my-token",
				Internal: &InternalIngressClassConfig{
					LoadBalancerConfigurationPath: "/opt",
				},
				External: nil,
			},
			args: args{
				o: &TaskIngressPluginConfig{
					ID:            "1234",
					NomadEndpoint: "my-endpoint",
					NomadToken:    "my-token",
					Internal: &InternalIngressClassConfig{
						LoadBalancerConfigurationPath: "/opt",
					},
					External: nil,
				},
			},
			want: false,
		},
		{
			name: "test not equal inner structs",
			fields: fields{
				ID:            "123",
				NomadEndpoint: "my-endpoint",
				NomadToken:    "my-token",
				Internal: &InternalIngressClassConfig{
					LoadBalancerConfigurationPath: "/opt/lb",
				},
				External: nil,
			},
			args: args{
				o: &TaskIngressPluginConfig{
					ID:            "123",
					NomadEndpoint: "my-endpoint",
					NomadToken:    "my-token",
					Internal: &InternalIngressClassConfig{
						LoadBalancerConfigurationPath: "/opt",
					},
					External: nil,
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &TaskIngressPluginConfig{
				ID:            tt.fields.ID,
				NomadEndpoint: tt.fields.NomadEndpoint,
				NomadToken:    tt.fields.NomadToken,
				Internal:      tt.fields.Internal,
				External:      tt.fields.External,
			}
			assert.Equalf(t1, tt.want, t.Equal(tt.args.o), "Equal(%v)", tt.args.o)
		})
	}
}
