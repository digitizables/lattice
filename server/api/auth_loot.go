package api

import (
	"context"
	"net/http"

	"connectrpc.com/connect/v2"
	"google.golang.org/protobuf/types/known/timestamppb"

	operatorv1 "github.com/digitizables/lattice/gen/lattice/operator/v1"
	"github.com/digitizables/lattice/server/auth"
	"github.com/digitizables/lattice/server/generate"
	"github.com/digitizables/lattice/server/store"
)

func lootToProto(l store.Loot) *operatorv1.LootItem {
	return &operatorv1.LootItem{
		LootId:      l.ID,
		AgentId:     l.AgentID,
		Name:        l.Name,
		RemotePath:  l.RemotePath,
		ContentType: l.ContentType,
		Size:        l.SizeBytes,
		Sha256:      l.SHA256,
		TaskId:      l.TaskID,
		CreatedAt:   timestamppb.New(l.CreatedAt),
	}
}

func (s *Server) Login(
	ctx context.Context,
	req *operatorv1.LoginRequest,
) (*operatorv1.LoginResponse, error) {
	if s.auth == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, "auth not configured")
	}
	ip := ""
	if r, ok := ctx.Value(httpReqKey{}).(*http.Request); ok && r != nil {
		ip = auth.ClientIP(r)
	}
	tok, expires, err := s.auth.Login(ctx, req.GetUsername(), req.GetPassword(), ip)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err.Error())
	}
	return &operatorv1.LoginResponse{
		AccessToken:      tok,
		Username:         req.GetUsername(),
		ExpiresInSeconds: expires,
	}, nil
}

func (s *Server) Logout(
	ctx context.Context,
	_ *operatorv1.LogoutRequest,
) (*operatorv1.LogoutResponse, error) {
	if s.auth != nil {
		if r, ok := ctx.Value(httpReqKey{}).(*http.Request); ok {
			s.auth.Logout(auth.TokenFromRequest(r))
		}
	}
	return &operatorv1.LogoutResponse{}, nil
}

func (s *Server) SetSessionNote(
	ctx context.Context,
	req *operatorv1.SetSessionNoteRequest,
) (*operatorv1.SetSessionNoteResponse, error) {
	if req.GetAgentId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, "agent_id is required")
	}
	agent, err := s.core.SetSessionNote(ctx, req.GetAgentId(), req.GetNote())
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err.Error())
	}
	return &operatorv1.SetSessionNoteResponse{Session: agentToSession(agent)}, nil
}

func (s *Server) SetAgentSleep(
	ctx context.Context,
	req *operatorv1.SetAgentSleepRequest,
) (*operatorv1.SetAgentSleepResponse, error) {
	if req.GetAgentId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, "agent_id is required")
	}
	agent, err := s.core.SetAgentSleep(ctx, req.GetAgentId(), req.GetSleepSeconds(), req.GetJitterPercent())
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err.Error())
	}
	return &operatorv1.SetAgentSleepResponse{Session: agentToSession(agent)}, nil
}

func (s *Server) ListLoot(
	ctx context.Context,
	req *operatorv1.ListLootRequest,
) (*operatorv1.ListLootResponse, error) {
	page, err := s.core.ListLoot(ctx, req.GetAgentId(), req.GetPageSize(), req.GetPageToken())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error())
	}
	out := &operatorv1.ListLootResponse{
		NextPageToken: page.NextPageToken,
		TotalCount:    page.TotalCount,
	}
	for _, it := range page.Items {
		out.Items = append(out.Items, lootToProto(it))
	}
	return out, nil
}

func (s *Server) GetLoot(
	ctx context.Context,
	req *operatorv1.GetLootRequest,
) (*operatorv1.GetLootResponse, error) {
	if req.GetLootId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, "loot_id is required")
	}
	item, data, err := s.core.GetLoot(ctx, req.GetLootId(), req.GetIncludeData())
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err.Error())
	}
	return &operatorv1.GetLootResponse{Item: lootToProto(item), Data: data}, nil
}

func (s *Server) DeleteLoot(
	ctx context.Context,
	req *operatorv1.DeleteLootRequest,
) (*operatorv1.DeleteLootResponse, error) {
	if req.GetLootId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, "loot_id is required")
	}
	if err := s.core.DeleteLoot(ctx, req.GetLootId()); err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err.Error())
	}
	return &operatorv1.DeleteLootResponse{}, nil
}

func (s *Server) GenerateAgent(
	ctx context.Context,
	req *operatorv1.GenerateAgentRequest,
) (*operatorv1.GenerateAgentResponse, error) {
	token := ""
	if req.GetEmbedAgentToken() {
		token = s.agentToken
	}
	res, err := generate.Build(ctx, generate.Request{
		ServerURL:       req.GetServerUrl(),
		OS:              req.GetOs(),
		Arch:            req.GetArch(),
		SleepSeconds:    req.GetSleepSeconds(),
		JitterPercent:   req.GetJitterPercent(),
		InsecureTLS:     req.GetInsecureTls(),
		AgentToken:      token,
		EmbedAgentToken: req.GetEmbedAgentToken(),
		OutputName:      req.GetOutputName(),
		OutDir:          s.generatedDir,
		ModuleRoot:      s.moduleRoot,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error())
	}
	return &operatorv1.GenerateAgentResponse{
		Path:   res.Path,
		Os:     res.OS,
		Arch:   res.Arch,
		Size:   res.Size,
		Sha256: res.SHA256,
		Data:   res.Data,
	}, nil
}

type httpReqKey struct{}

// WithHTTPRequest attaches the inbound *http.Request for Login rate limiting.
func WithHTTPRequest(ctx context.Context, r *http.Request) context.Context {
	return context.WithValue(ctx, httpReqKey{}, r)
}
