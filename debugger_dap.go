package frankenphp

import (
	"bufio"
	"fmt"
	"log/slog"
	"net"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/google/go-dap"
)

type varRef struct {
	value any // []DebugVariable for scopes, map[string]any or []any for expanded children
}

type exceptionCatchMode int

const (
	catchNone     exceptionCatchMode = 0
	catchUncaught exceptionCatchMode = 1
	catchCaught   exceptionCatchMode = 2
)

type dapServer struct {
	listener   net.Listener
	mu         sync.Mutex
	conn       net.Conn
	writer     *bufio.Writer
	seq        atomic.Int32
	bpIDs      map[string][]int // filename -> breakpoint IDs
	stopCh     chan struct{}
	varMu      sync.Mutex
	nextVarRef int
	varRefs    map[int]*varRef
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
		varRefs:  make(map[int]*varRef),
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
	outChan := SubscribeOutput()
	for {
		select {
		case <-s.stopCh:
			return
		case hit, ok := <-bpChan:
			if !ok {
				return
			}
			threadId := hit.ThreadIndex + 1

			info := getThreadDebugInfo(hit.ThreadIndex)
			reason := info.StopReason
			if reason == "" {
				reason = "breakpoint"
			}

			body := dap.StoppedEventBody{
				Reason:            reason,
				ThreadId:          threadId,
				AllThreadsStopped: false,
			}
			if reason == "exception" {
				body.Description = info.ExceptionClass
				body.Text = info.ExceptionClass + ": " + info.ExceptionMessage
			}

			s.sendEvent(&dap.StoppedEvent{
				Event: *s.newEvent("stopped"),
				Body:  body,
			})
		case msg, ok := <-outChan:
			if !ok {
				return
			}
			s.sendEvent(&dap.OutputEvent{
				Event: *s.newEvent("output"),
				Body: dap.OutputEventBody{
					Category: "console",
					Output:   msg.Output + "\n",
				},
			})
		}
	}
}

func (s *dapServer) allocVarRef(value any) int {
	s.varMu.Lock()
	defer s.varMu.Unlock()
	s.nextVarRef++
	s.varRefs[s.nextVarRef] = &varRef{value: value}
	return s.nextVarRef
}

func (s *dapServer) getVarRef(ref int) *varRef {
	s.varMu.Lock()
	defer s.varMu.Unlock()
	return s.varRefs[ref]
}

func (s *dapServer) clearVarRefs() {
	s.varMu.Lock()
	defer s.varMu.Unlock()
	s.varRefs = make(map[int]*varRef)
	s.nextVarRef = 0
}

func (s *dapServer) handleConnection(conn net.Conn) {
	s.clearVarRefs()
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
			SupportsConfigurationDoneRequest:  true,
			SupportsFunctionBreakpoints:       false,
			SupportsConditionalBreakpoints:    true,
			SupportsHitConditionalBreakpoints: true,
			SupportsLogPoints:                 true,
			SupportsEvaluateForHovers:         false,
			SupportsStepBack:                  false,
			SupportsSetVariable:               false,
			SupportsRestartFrame:              false,
			SupportsStepInTargetsRequest:      false,
			SupportsDelayedStackTraceLoading:  false,
			ExceptionBreakpointFilters: []dap.ExceptionBreakpointsFilter{
				{
					Filter:  "uncaught",
					Label:   "Uncaught Exceptions",
					Default: true,
				},
				{
					Filter:  "caught",
					Label:   "Caught Exceptions",
					Default: false,
				},
			},
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
		id := SetBreakpoint(file, bp.Line, bp.Condition, bp.HitCondition, bp.LogMessage)
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
	hasCaught := false
	hasUncaught := false
	for _, f := range req.Arguments.Filters {
		switch f {
		case "caught":
			hasCaught = true
		case "uncaught":
			hasUncaught = true
		}
	}

	mode := catchNone
	if hasUncaught {
		mode |= catchUncaught
	}
	if hasCaught {
		mode |= catchCaught
	}
	SetExceptionBreakMode(mode)

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
	frameId := req.Arguments.FrameId
	threadIndex := frameId/1000 - 1 // convert 1-based DAP threadId back to 0-based
	frameIndex := frameId % 1000

	vars := GetFrameVariables(threadIndex, frameIndex)
	localsRef := 0
	if len(vars) > 0 {
		localsRef = s.allocVarRef(vars)
	}

	globals := GetGlobals(threadIndex)
	globalsRef := 0
	if len(globals) > 0 {
		globalsRef = s.allocVarRef(globals)
	}

	s.sendResponse(&dap.ScopesResponse{
		Response: *s.newResponse(req.Seq, req.Command),
		Body: dap.ScopesResponseBody{
			Scopes: []dap.Scope{
				{
					Name:               "Locals",
					VariablesReference: localsRef,
					Expensive:          false,
				},
				{
					Name:               "Globals",
					VariablesReference: globalsRef,
					Expensive:          false,
				},
			},
		},
	})
}

func (s *dapServer) onVariables(req *dap.VariablesRequest) {
	vr := s.getVarRef(req.Arguments.VariablesReference)
	if vr == nil {
		s.sendResponse(&dap.VariablesResponse{
			Response: *s.newResponse(req.Seq, req.Command),
			Body:     dap.VariablesResponseBody{Variables: []dap.Variable{}},
		})
		return
	}

	var dapVars []dap.Variable

	switch val := vr.value.(type) {
	case []DebugVariable:
		dapVars = make([]dap.Variable, len(val))
		for i, dv := range val {
			childRef := 0
			if isExpandable(dv.Value) {
				childRef = s.allocVarRef(dv.Value)
			}
			dapVars[i] = dap.Variable{
				Name:               dv.Name,
				Value:              formatDebugVar(dv),
				Type:               dv.Type,
				VariablesReference: childRef,
			}
		}

	case DebugObject:
		dapVars = make([]dap.Variable, len(val.Properties))
		for i, dv := range val.Properties {
			childRef := 0
			if isExpandable(dv.Value) {
				childRef = s.allocVarRef(dv.Value)
			}
			dapVars[i] = dap.Variable{
				Name:               dv.Name,
				Value:              formatDebugVar(dv),
				Type:               dv.Type,
				VariablesReference: childRef,
			}
		}

	case AssociativeArray[any]:
		dapVars = make([]dap.Variable, len(val.Order))
		for i, k := range val.Order {
			child := val.Map[k]
			childRef := 0
			if isExpandable(child) {
				childRef = s.allocVarRef(child)
			}
			dapVars[i] = dap.Variable{
				Name:               k,
				Value:              formatValue(child),
				Type:               goTypeToPhpType(child),
				VariablesReference: childRef,
			}
		}

	case map[string]any:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		dapVars = make([]dap.Variable, len(keys))
		for i, k := range keys {
			child := val[k]
			childRef := 0
			if isExpandable(child) {
				childRef = s.allocVarRef(child)
			}
			dapVars[i] = dap.Variable{
				Name:               k,
				Value:              formatValue(child),
				Type:               goTypeToPhpType(child),
				VariablesReference: childRef,
			}
		}

	case []any:
		dapVars = make([]dap.Variable, len(val))
		for i, child := range val {
			childRef := 0
			if isExpandable(child) {
				childRef = s.allocVarRef(child)
			}
			dapVars[i] = dap.Variable{
				Name:               fmt.Sprintf("[%d]", i),
				Value:              formatValue(child),
				Type:               goTypeToPhpType(child),
				VariablesReference: childRef,
			}
		}
	}

	s.sendResponse(&dap.VariablesResponse{
		Response: *s.newResponse(req.Seq, req.Command),
		Body:     dap.VariablesResponseBody{Variables: dapVars},
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
	s.clearVarRefs()
	SetExceptionBreakMode(catchNone)

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

func isExpandable(v any) bool {
	switch v.(type) {
	case map[string]any, []any, AssociativeArray[any], DebugObject, []DebugVariable:
		return true
	}
	return false
}

func formatDebugVar(v DebugVariable) string {
	if v.Type == "undef" {
		return "undefined"
	}
	return formatValue(v.Value)
}

func formatValue(v any) string {
	if v == nil {
		return "null"
	}
	switch val := v.(type) {
	case bool:
		if val {
			return "true"
		}
		return "false"
	case int:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%g", val)
	case string:
		if len(val) > 80 {
			return fmt.Sprintf("\"%s...\"", val[:80])
		}
		return fmt.Sprintf("\"%s\"", val)
	case DebugObject:
		return val.ClassName
	case DebugResource:
		return fmt.Sprintf("resource #%d (%s)", val.ID, val.TypeName)
	case []DebugVariable:
		return fmt.Sprintf("array(%d)", len(val))
	case AssociativeArray[any]:
		return fmt.Sprintf("array(%d)", len(val.Map))
	case map[string]any:
		return fmt.Sprintf("array(%d)", len(val))
	case []any:
		return fmt.Sprintf("array(%d)", len(val))
	default:
		return fmt.Sprintf("%v", v)
	}
}

func goTypeToPhpType(v any) string {
	if v == nil {
		return "null"
	}
	switch v.(type) {
	case bool:
		return "bool"
	case int, int64:
		return "int"
	case float64:
		return "float"
	case string:
		return "string"
	case DebugObject:
		return "object"
	case []DebugVariable, AssociativeArray[any], map[string]any, []any:
		return "array"
	default:
		return "mixed"
	}
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
