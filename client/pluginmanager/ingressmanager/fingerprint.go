package ingressmanager

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/dynamicplugins"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/ingress"
)

type pluginFingerprinter struct {
	logger hclog.Logger
	client ingress.IngressPlugin
	info   *dynamicplugins.PluginInfo

	basicInfo *structs.IngressInfo

	hadFirstSuccessfulFingerprint bool
	// hadFirstSuccessfulFingerprintCh is closed the first time a fingerprint
	// is completed successfully.
	hadFirstSuccessfulFingerprintCh chan struct{}

	// requiresStaging is set on a first successful fingerprint. It allows the
	// ingressmanager to efficiently query this as it shouldn't change after a plugin
	// is started. Removing this bool will require storing a cache of recent successful
	// results that can be used by subscribers of the `hadFirstSuccessfulFingerprintCh`.
	requiresStaging bool
}

func (p *pluginFingerprinter) fingerprint(ctx context.Context) *structs.IngressInfo {
	if p.basicInfo == nil {
		info := p.buildBasicFingerprint(ctx)
		p.basicInfo = info
	}

	info := p.basicInfo.Copy()
	var fp *structs.IngressInfo
	var err error

	fp, err = p.buildControllerFingerprint(ctx, info)

	if err != nil {
		info.Healthy = false
		info.HealthDescription = fmt.Sprintf("failed fingerprinting with error: %v", err)
	} else {
		info = fp
		if !p.hadFirstSuccessfulFingerprint {
			p.hadFirstSuccessfulFingerprint = true
			close(p.hadFirstSuccessfulFingerprintCh)
		}
	}

	return info
}

func (p *pluginFingerprinter) buildBasicFingerprint(ctx context.Context) *structs.IngressInfo {
	return &structs.IngressInfo{
		PluginID:          p.info.Name,
		AllocID:           p.info.AllocID,
		Provider:          p.info.Options["Provider"],
		ProviderVersion:   p.info.Version,
		Healthy:           false,
		HealthDescription: "initial fingerprint not completed",
	}
}

func (p *pluginFingerprinter) buildControllerFingerprint(ctx context.Context, base *structs.IngressInfo) (*structs.IngressInfo, error) {
	fp := base.Copy()

	healthy, err := p.client.PluginProbe(ctx)
	if err != nil {
		return nil, err
	}
	fp.SetHealthy(healthy)

	return fp, nil
}
