package api

import (
	"context"
	"log/slog"

	"connectrpc.com/connect/v2"

	operatorv1 "github.com/digitizables/lattice/gen/lattice/operator/v1"
	"github.com/digitizables/lattice/gen/lattice/operator/v1/operatorv1connect"
	"github.com/digitizables/lattice/server/core"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Server implements OperatorService.
type Server struct {
	core *core.Core
	log  *slog.Logger
}

// New returns an OperatorService handler wired to core.
func New(c *core.Core, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	return &Server{core: c, log: log}
}

// Register mounts the operator service on a Connect server.
func (s *Server) Register(srv *connect.Server) {
	operatorv1connect.RegisterOperatorServiceHandler(srv, s)
}

func (s *Server) ListSessions(
	ctx context.Context,
	_ *operatorv1.ListSessionsRequest,
) (*operatorv1.ListSessionsResponse, error) {
	agents, err := s.core.ListSessions(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error())
	}
	resp := &operatorv1.ListSessionsResponse{}
	for _, a := range agents {
		resp.Sessions = append(resp.Sessions, &operatorv1.Session{
			AgentId:  a.ID,
			Hostname: a.Hostname,
			Username: a.Username,
			Os:       a.OS,
			Arch:     a.Arch,
			LastSeen: timestamppb.New(a.LastSeen),
		})
	}
	return resp, nil
}

func (s *Server) SubmitTask(
	ctx context.Context,
	req *operatorv1.SubmitTaskRequest,
) (*operatorv1.SubmitTaskResponse, error) {
	if req.GetAgentId() == "" || req.GetName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, "agent_id and name are required")
	}
	taskID, err := s.core.SubmitTask(ctx, req.GetAgentId(), req.GetName(), req.GetArgs())
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err.Error())
	}
	return &operatorv1.SubmitTaskResponse{TaskId: taskID}, nil
}

func (s *Server) StreamEvents(
	_ context.Context,
	_ *operatorv1.StreamEventsRequest,
	_ operatorv1connect.OperatorServiceStreamEventsServerStream,
) error {
	return connect.NewError(connect.CodeUnimplemented, "StreamEvents not implemented yet")
}

var _ operatorv1connect.OperatorServiceHandler = (*Server)(nil)
