package agent

import (
	"github.com/hashicorp/nomad/nomad/structs"
	"net/http"
)

func (s *HTTPServer) IngressPluginsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != http.MethodGet {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	args := structs.IngressPluginListRequest{}

	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.IngressPluginListResponse
	if err := s.agent.RPC("IngressPlugin.List", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	return out.Plugins, nil
}

func (s *HTTPServer) IngressPluginSpecificRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != http.MethodGet {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	id := req.URL.Query().Get("id")

	args := structs.IngressPluginGetRequest{ID: id}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.IngressPluginGetResponse
	if err := s.agent.RPC("IngressPlugin.Get", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Plugin == nil {
		return nil, CodedError(404, "plugin not found")
	}

	return out.Plugin, nil
}
