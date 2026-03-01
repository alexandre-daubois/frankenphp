package frankenphp

import (
	"bufio"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"

	"github.com/google/go-dap"
)

type dapServer struct {
	listener net.Listener
	mu       sync.Mutex
	conn     net.Conn
	writer   *bufio.Writer
	seq      atomic.Int32
	bpIDs    map[string][]int // filename -> breakpoint IDs
	stopCh   chan struct{}
}

var debugServer *dapServer

func startDebugServer(listen string) error {
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return fmt.Errorf("debugger: failed to listen on %s: %w", listen, err)
	}

	debugDAPListen = ln.Addr().String()

	debugServer = &dapServer{
		listener: ln,
		bpIDs:    make(map[string][]int),
		stopCh:   make(chan struct{}),
	}

	go debugServer.acceptLoop()
	go debugServer.eventLoop()

	return nil
}

func (s *dapServer) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.stopCh:
				return
			default:
				globalLogger.LogAttrs(globalCtx, slog.LevelError, "debugger: accept error", slog.Any("error", err))
				continue
			}
		}

		s.mu.Lock()
		if s.conn != nil {
			s.conn.Close()
		}
		s.conn = conn
		s.writer = bufio.NewWriter(conn)
		s.mu.Unlock()

		globalLogger.LogAttrs(globalCtx, slog.LevelInfo, "debugger: client connected", slog.String("remote", conn.RemoteAddr().String()))
		s.handleConnection(conn)
	}
}

func (s *dapServer) eventLoop() {
	bpChan := SubscribeBreakpoints()
	for {
		select {
		case <-s.stopCh:
			return
		case hit, ok := <-bpChan:
			if !ok {
				return
			}
			threadId := hit.ThreadIndex + 1 // DAP thread IDs are 1-based
			s.sendEvent(&dap.StoppedEvent{
				Event: *s.newEvent("stopped"),
				Body: dap.StoppedEventBody{
					Reason:            "breakpoint",
					ThreadId:          threadId,
					AllThreadsStopped: false,
				},
			})
		}
	}
}

func (s *dapServer) handleConnection(conn net.Conn) {
	reader := bufio.NewReader(conn)
	for {
		msg, err := dap.ReadProtocolMessage(reader)
		if err != nil {
			globalLogger.LogAttrs(globalCtx, slog.LevelDebug, "debugger: read error (client disconnected?)", slog.Any("error", err))
			return
		}

		switch req := msg.(type) {
		case *dap.InitializeRequest:
			s.onInitialize(req)
		case *dap.LaunchRequest:
			s.onLaunch(req)
		case *dap.AttachRequest:
			s.onAttach(req)
		case *dap.SetBreakpointsRequest:
			s.onSetBreakpoints(req)
		case *dap.SetExceptionBreakpointsRequest:
			s.onSetExceptionBreakpoints(req)
		case *dap.ConfigurationDoneRequest:
			s.onConfigurationDone(req)
		case *dap.ThreadsRequest:
			s.onThreads(req)
		case *dap.StackTraceRequest:
			s.onStackTrace(req)
		case *dap.ScopesRequest:
			s.onScopes(req)
		case *dap.VariablesRequest:
			s.onVariables(req)
		case *dap.ContinueRequest:
			s.onContinue(req)
		case *dap.NextRequest:
			s.onNext(req)
		case *dap.StepInRequest:
			s.onStepIn(req)
		case *dap.StepOutRequest:
			s.onStepOut(req)
		case *dap.PauseRequest:
			s.onPause(req)
		case *dap.DisconnectRequest:
			s.onDisconnect(req)
			return
		case *dap.EvaluateRequest:
			s.onEvaluate(req)
		default:
			globalLogger.LogAttrs(globalCtx, slog.LevelDebug, "debugger: unhandled DAP message", slog.String("type", fmt.Sprintf("%T", msg)))
		}
	}
}

func (s *dapServer) onInitialize(req *dap.InitializeRequest) {
	s.sendResponse(&dap.InitializeResponse{
		Response: *s.newResponse(req.Seq, req.Command),
		Body: dap.Capabilities{
			SupportsConfigurationDoneRequest: true,
			SupportsFunctionBreakpoints:      false,
			SupportsConditionalBreakpoints:   false,
			SupportsEvaluateForHovers:        false,
			SupportsStepBack:                 false,
			SupportsSetVariable:              false,
			SupportsRestartFrame:             false,
			SupportsStepInTargetsRequest:     false,
			SupportsDelayedStackTraceLoading: false,
		},
	})
	s.sendEvent(&dap.InitializedEvent{
		Event: *s.newEvent("initialized"),
	})
}

func (s *dapServer) onLaunch(req *dap.LaunchRequest) {
	s.sendResponse(&dap.LaunchResponse{
		Response: *s.newResponse(req.Seq, req.Command),
	})
}

func (s *dapServer) onAttach(req *dap.AttachRequest) {
	s.sendResponse(&dap.AttachResponse{
		Response: *s.newResponse(req.Seq, req.Command),
	})
}

func (s *dapServer) onSetBreakpoints(req *dap.SetBreakpointsRequest) {
	file := req.Arguments.Source.Path

	if oldIDs, ok := s.bpIDs[file]; ok {
		for _, id := range oldIDs {
			RemoveBreakpoint(id)
		}
	}

	bps := make([]dap.Breakpoint, len(req.Arguments.Breakpoints))
	newIDs := make([]int, len(req.Arguments.Breakpoints))
	for i, bp := range req.Arguments.Breakpoints {
		id := SetBreakpoint(file, bp.Line)
		newIDs[i] = id
		bps[i] = dap.Breakpoint{
			Id:       id,
			Verified: true,
			Line:     bp.Line,
			Source:   &req.Arguments.Source,
		}
	}
	s.bpIDs[file] = newIDs

	s.sendResponse(&dap.SetBreakpointsResponse{
		Response: *s.newResponse(req.Seq, req.Command),
		Body: dap.SetBreakpointsResponseBody{
			Breakpoints: bps,
		},
	})
}

func (s *dapServer) onSetExceptionBreakpoints(req *dap.SetExceptionBreakpointsRequest) {
	s.sendResponse(&dap.SetExceptionBreakpointsResponse{
		Response: *s.newResponse(req.Seq, req.Command),
	})
}

func (s *dapServer) onConfigurationDone(req *dap.ConfigurationDoneRequest) {
	s.sendResponse(&dap.ConfigurationDoneResponse{
		Response: *s.newResponse(req.Seq, req.Command),
	})
}

func (s *dapServer) onThreads(req *dap.ThreadsRequest) {
	dbgState := DebugState()
	threads := make([]dap.Thread, 0, len(dbgState.ThreadDebugStates))
	for _, t := range dbgState.ThreadDebugStates {
		threads = append(threads, dap.Thread{
			Id:   t.Index + 1, // DAP thread IDs are 1-based
			Name: t.Name,
		})
	}

	s.sendResponse(&dap.ThreadsResponse{
		Response: *s.newResponse(req.Seq, req.Command),
		Body: dap.ThreadsResponseBody{
			Threads: threads,
		},
	})
}

func (s *dapServer) onStackTrace(req *dap.StackTraceRequest) {
	threadIndex := req.Arguments.ThreadId - 1 // convert 1-based DAP ID to 0-based index
	frames := GetStackTrace(threadIndex)

	dapFrames := make([]dap.StackFrame, len(frames))
	for i, f := range frames {
		name := f.Function
		if f.Class != "" {
			name = f.Class + "::" + f.Function
		}
		dapFrames[i] = dap.StackFrame{
			Id:   req.Arguments.ThreadId*1000 + i,
			Name: name,
			Source: &dap.Source{
				Name: f.File,
				Path: f.File,
			},
			Line:   f.Line,
			Column: 1,
		}
	}

	s.sendResponse(&dap.StackTraceResponse{
		Response: *s.newResponse(req.Seq, req.Command),
		Body: dap.StackTraceResponseBody{
			StackFrames: dapFrames,
			TotalFrames: len(dapFrames),
		},
	})
}

func (s *dapServer) onScopes(req *dap.ScopesRequest) {
	// frameId encodes threadIndex*1000 + frameIndex
	frameId := req.Arguments.FrameId

	s.sendResponse(&dap.ScopesResponse{
		Response: *s.newResponse(req.Seq, req.Command),
		Body: dap.ScopesResponseBody{
			Scopes: []dap.Scope{
				{
					Name:               "Locals",
					VariablesReference: frameId*10 + 1,
					Expensive:          false,
				},
			},
		},
	})
}

func (s *dapServer) onVariables(req *dap.VariablesRequest) {
	// variablesReference encodes frameId*10 + scopeType
	// For now we only return locals (scopeType 1)
	ref := req.Arguments.VariablesReference
	threadIndex := ref / 10000

	vars := GetLocals(threadIndex)

	dapVars := make([]dap.Variable, len(vars))
	for i, v := range vars {
		dapVars[i] = dap.Variable{
			Name:  v.Name,
			Value: fmt.Sprintf("%v", v.Value),
			Type:  v.Type,
		}
	}

	s.sendResponse(&dap.VariablesResponse{
		Response: *s.newResponse(req.Seq, req.Command),
		Body: dap.VariablesResponseBody{
			Variables: dapVars,
		},
	})
}

func (s *dapServer) onContinue(req *dap.ContinueRequest) {
	ContinueThread(req.Arguments.ThreadId - 1)
	s.sendResponse(&dap.ContinueResponse{
		Response: *s.newResponse(req.Seq, req.Command),
		Body: dap.ContinueResponseBody{
			AllThreadsContinued: false,
		},
	})
}

func (s *dapServer) onNext(req *dap.NextRequest) {
	StepOver(req.Arguments.ThreadId - 1)
	s.sendResponse(&dap.NextResponse{
		Response: *s.newResponse(req.Seq, req.Command),
	})
	s.sendEvent(&dap.ContinuedEvent{
		Event: *s.newEvent("continued"),
		Body: dap.ContinuedEventBody{
			ThreadId: req.Arguments.ThreadId,
		},
	})
}

func (s *dapServer) onStepIn(req *dap.StepInRequest) {
	StepInto(req.Arguments.ThreadId - 1)
	s.sendResponse(&dap.StepInResponse{
		Response: *s.newResponse(req.Seq, req.Command),
	})
	s.sendEvent(&dap.ContinuedEvent{
		Event: *s.newEvent("continued"),
		Body: dap.ContinuedEventBody{
			ThreadId: req.Arguments.ThreadId,
		},
	})
}

func (s *dapServer) onStepOut(req *dap.StepOutRequest) {
	StepOut(req.Arguments.ThreadId - 1)
	s.sendResponse(&dap.StepOutResponse{
		Response: *s.newResponse(req.Seq, req.Command),
	})
	s.sendEvent(&dap.ContinuedEvent{
		Event: *s.newEvent("continued"),
		Body: dap.ContinuedEventBody{
			ThreadId: req.Arguments.ThreadId,
		},
	})
}

func (s *dapServer) onPause(req *dap.PauseRequest) {
	// Not implemented yet, would need to interrupt a running thread
	s.sendResponse(&dap.PauseResponse{
		Response: *s.newResponse(req.Seq, req.Command),
	})
}

func (s *dapServer) onDisconnect(req *dap.DisconnectRequest) {
	debugMu.RLock()
	for idx := range debugThreadInfo {
		globalLogger.LogAttrs(globalCtx, slog.LevelDebug, "debugger: resumed thread on disconnect", slog.Int("thread", idx))
		ContinueThread(idx)
	}
	debugMu.RUnlock()

	s.sendResponse(&dap.DisconnectResponse{
		Response: *s.newResponse(req.Seq, req.Command),
	})
}

func (s *dapServer) onEvaluate(req *dap.EvaluateRequest) {
	// Not implemented yet, would need zend_eval_string
	s.sendErrorResponse(req.Seq, req.Command, "evaluate not supported yet")
}

func (s *dapServer) newResponse(requestSeq int, command string) *dap.Response {
	return &dap.Response{
		ProtocolMessage: dap.ProtocolMessage{
			Seq:  int(s.seq.Add(1)),
			Type: "response",
		},
		Command:    command,
		RequestSeq: requestSeq,
		Success:    true,
	}
}

func (s *dapServer) newEvent(event string) *dap.Event {
	return &dap.Event{
		ProtocolMessage: dap.ProtocolMessage{
			Seq:  int(s.seq.Add(1)),
			Type: "event",
		},
		Event: event,
	}
}

func (s *dapServer) sendResponse(msg dap.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return
	}
	err := dap.WriteProtocolMessage(s.writer, msg)
	if err != nil {
		globalLogger.LogAttrs(globalCtx, slog.LevelError, "debugger: write error", slog.Any("error", err))
		return
	}
	s.writer.Flush()
}

func (s *dapServer) sendEvent(msg dap.Message) {
	s.sendResponse(msg)
}

func (s *dapServer) sendErrorResponse(requestSeq int, command string, message string) {
	resp := s.newResponse(requestSeq, command)
	resp.Success = false
	resp.Message = message
	s.sendResponse(&dap.ErrorResponse{
		Response: *resp,
	})
}
