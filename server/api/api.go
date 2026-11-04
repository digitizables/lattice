package api

import (
	"context"
	"log/slog"

	"connectrpc.com/connect/v2"
	"google.golang.org/protobuf/types/known/timestamppb"

	operatorv1 "github.com/digitizables/lattice/gen/lattice/operator/v1"
	"github.com/digitizables/lattice/gen/lattice/operator/v1/operatorv1connect"
	"github.com/digitizables/lattice/server/core"
	"github.com/digitizables/lattice/server/store"
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
		resp.Sessions = append(resp.Sessions, agentToSession(a))
	}
	return resp, nil
}

func (s *Server) DeleteSession(
	ctx context.Context,
	req *operatorv1.DeleteSessionRequest,
) (*operatorv1.DeleteSessionResponse, error) {
	if req.GetAgentId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, "agent_id is required")
	}
	if err := s.core.DeleteSession(ctx, req.GetAgentId()); err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err.Error())
	}
	return &operatorv1.DeleteSessionResponse{}, nil
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
	ctx context.Context,
	req *operatorv1.StreamEventsRequest,
	stream operatorv1connect.OperatorServiceStreamEventsServerStream,
) error {
	// Flush headers so clients can subscribe before the first event (avoids deadlock with submit-then-wait).
	if err := stream.SendHeaders(); err != nil {
		return err
	}

	filterAgent := req.GetAgentId()
	events, unsub := s.core.SubscribeEvents(64)
	defer unsub()

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-events:
			if !ok {
				return nil
			}
			msg := eventToProto(ev)
			if msg == nil {
				continue
			}
			if filterAgent != "" {
				switch p := msg.Payload.(type) {
				case *operatorv1.StreamEventsResponse_SessionUpdated:
					if p.SessionUpdated.GetSession().GetAgentId() != filterAgent {
						continue
					}
				case *operatorv1.StreamEventsResponse_TaskUpdated:
					if p.TaskUpdated.GetAgentId() != filterAgent {
						continue
					}
				}
			}
			if err := stream.Send(msg); err != nil {
				return err
			}
		}
	}
}

func agentToSession(a store.Agent) *operatorv1.Session {
	return &operatorv1.Session{
		AgentId:  a.ID,
		Hostname: a.Hostname,
		Username: a.Username,
		Os:       a.OS,
		Arch:     a.Arch,
		LastSeen: timestamppb.New(a.LastSeen),
	}
}

func eventToProto(ev core.Event) *operatorv1.StreamEventsResponse {
	switch {
	case ev.Session != nil:
		return &operatorv1.StreamEventsResponse{
			Payload: &operatorv1.StreamEventsResponse_SessionUpdated{
				SessionUpdated: &operatorv1.SessionUpdated{
					Session: agentToSession(*ev.Session),
				},
			},
		}
	case ev.Task != nil:
		t := ev.Task
		var exit int32
		if t.ExitCode != nil {
			exit = *t.ExitCode
		}
		return &operatorv1.StreamEventsResponse{
			Payload: &operatorv1.StreamEventsResponse_TaskUpdated{
				TaskUpdated: &operatorv1.TaskUpdated{
					TaskId:   t.ID,
					AgentId:  t.AgentID,
					Status:   t.Status,
					Stdout:   t.Stdout,
					Stderr:   t.Stderr,
					ExitCode: exit,
				},
			},
		}
	default:
		return nil
	}
}

var _ operatorv1connect.OperatorServiceHandler = (*Server)(nil)
