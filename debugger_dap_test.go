package frankenphp_test

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dunglas/frankenphp"
	"github.com/google/go-dap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDAPServerHandshake(t *testing.T) {
	opts := debuggerTestOpts()
	runTest(t, func(_ func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		status := frankenphp.DebuggerStatus()
		require.True(t, status.Enabled)
		require.NotEmpty(t, status.DAPListen)

		conn, err := net.DialTimeout("tcp", status.DAPListen, 2*time.Second)
		require.NoError(t, err)
		defer conn.Close()

		writer := bufio.NewWriter(conn)
		reader := bufio.NewReader(conn)

		// Send InitializeRequest
		seq := 1
		initReq := &dap.InitializeRequest{
			Request: dap.Request{
				ProtocolMessage: dap.ProtocolMessage{Seq: seq, Type: "request"},
				Command:         "initialize",
			},
			Arguments: dap.InitializeRequestArguments{
				AdapterID: "test",
			},
		}
		err = dap.WriteProtocolMessage(writer, initReq)
		require.NoError(t, err)
		err = writer.Flush()
		require.NoError(t, err)

		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		msg1, err := dap.ReadProtocolMessage(reader)
		require.NoError(t, err)
		initResp, ok := msg1.(*dap.InitializeResponse)
		require.True(t, ok, "expected InitializeResponse, got %T", msg1)
		assert.True(t, initResp.Success)

		msg2, err := dap.ReadProtocolMessage(reader)
		require.NoError(t, err)
		_, ok = msg2.(*dap.InitializedEvent)
		assert.True(t, ok, "expected InitializedEvent, got %T", msg2)

		seq++
		disconnReq := &dap.DisconnectRequest{
			Request: dap.Request{
				ProtocolMessage: dap.ProtocolMessage{Seq: seq, Type: "request"},
				Command:         "disconnect",
			},
		}
		err = dap.WriteProtocolMessage(writer, disconnReq)
		require.NoError(t, err)
		err = writer.Flush()
		require.NoError(t, err)

		msg3, err := dap.ReadProtocolMessage(reader)
		require.NoError(t, err)
		disconnResp, ok := msg3.(*dap.DisconnectResponse)
		assert.True(t, ok, "expected DisconnectResponse, got %T", msg3)
		if ok {
			assert.True(t, disconnResp.Success)
		}
	}, opts)
}

// dapSend writes a DAP message and flushes the writer.
func dapSend(t *testing.T, writer *bufio.Writer, msg dap.Message) {
	t.Helper()
	err := dap.WriteProtocolMessage(writer, msg)
	require.NoError(t, err)
	require.NoError(t, writer.Flush())
}

// dapRecv reads the next DAP message with a deadline.
func dapRecv(t *testing.T, conn net.Conn, reader *bufio.Reader) dap.Message {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	msg, err := dap.ReadProtocolMessage(reader)
	require.NoError(t, err)
	return msg
}

func TestDAPVariables(t *testing.T) {
	opts := debuggerTestOpts()
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		status := frankenphp.DebuggerStatus()
		require.True(t, status.Enabled)
		require.NotEmpty(t, status.DAPListen)

		conn, err := net.DialTimeout("tcp", status.DAPListen, 2*time.Second)
		require.NoError(t, err)
		defer conn.Close()

		writer := bufio.NewWriter(conn)
		reader := bufio.NewReader(conn)
		seq := 0

		nextSeq := func() int {
			seq++
			return seq
		}

		dapSend(t, writer, &dap.InitializeRequest{
			Request: dap.Request{
				ProtocolMessage: dap.ProtocolMessage{Seq: nextSeq(), Type: "request"},
				Command:         "initialize",
			},
			Arguments: dap.InitializeRequestArguments{AdapterID: "test"},
		})
		msg := dapRecv(t, conn, reader)
		_, ok := msg.(*dap.InitializeResponse)
		require.True(t, ok, "expected InitializeResponse, got %T", msg)
		msg = dapRecv(t, conn, reader)
		_, ok = msg.(*dap.InitializedEvent)
		require.True(t, ok, "expected InitializedEvent, got %T", msg)

		// SetBreakpoints on debugger-variables.php line 15
		cwd, _ := os.Getwd()
		bpFile := filepath.Join(cwd, "testdata", "debugger-variables.php")
		dapSend(t, writer, &dap.SetBreakpointsRequest{
			Request: dap.Request{
				ProtocolMessage: dap.ProtocolMessage{Seq: nextSeq(), Type: "request"},
				Command:         "setBreakpoints",
			},
			Arguments: dap.SetBreakpointsArguments{
				Source:      dap.Source{Path: bpFile},
				Breakpoints: []dap.SourceBreakpoint{{Line: 15}},
			},
		})
		msg = dapRecv(t, conn, reader)
		bpResp, ok := msg.(*dap.SetBreakpointsResponse)
		require.True(t, ok, "expected SetBreakpointsResponse, got %T", msg)
		require.Len(t, bpResp.Body.Breakpoints, 1)
		assert.True(t, bpResp.Body.Breakpoints[0].Verified)

		dapSend(t, writer, &dap.ConfigurationDoneRequest{
			Request: dap.Request{
				ProtocolMessage: dap.ProtocolMessage{Seq: nextSeq(), Type: "request"},
				Command:         "configurationDone",
			},
		})
		msg = dapRecv(t, conn, reader)
		_, ok = msg.(*dap.ConfigurationDoneResponse)
		require.True(t, ok, "expected ConfigurationDoneResponse, got %T", msg)

		done := make(chan string, 1)
		go func() {
			body, _ := testGet("http://example.com/debugger-variables.php", handler, t)
			done <- body
		}()

		msg = dapRecv(t, conn, reader)
		stopped, ok := msg.(*dap.StoppedEvent)
		require.True(t, ok, "expected StoppedEvent, got %T", msg)
		assert.Equal(t, "breakpoint", stopped.Body.Reason)
		threadId := stopped.Body.ThreadId

		dapSend(t, writer, &dap.ThreadsRequest{
			Request: dap.Request{
				ProtocolMessage: dap.ProtocolMessage{Seq: nextSeq(), Type: "request"},
				Command:         "threads",
			},
		})
		msg = dapRecv(t, conn, reader)
		threadsResp, ok := msg.(*dap.ThreadsResponse)
		require.True(t, ok, "expected ThreadsResponse, got %T", msg)
		assert.NotEmpty(t, threadsResp.Body.Threads)

		dapSend(t, writer, &dap.StackTraceRequest{
			Request: dap.Request{
				ProtocolMessage: dap.ProtocolMessage{Seq: nextSeq(), Type: "request"},
				Command:         "stackTrace",
			},
			Arguments: dap.StackTraceArguments{ThreadId: threadId},
		})
		msg = dapRecv(t, conn, reader)
		stackResp, ok := msg.(*dap.StackTraceResponse)
		require.True(t, ok, "expected StackTraceResponse, got %T", msg)
		require.NotEmpty(t, stackResp.Body.StackFrames)
		frameId := stackResp.Body.StackFrames[0].Id

		dapSend(t, writer, &dap.ScopesRequest{
			Request: dap.Request{
				ProtocolMessage: dap.ProtocolMessage{Seq: nextSeq(), Type: "request"},
				Command:         "scopes",
			},
			Arguments: dap.ScopesArguments{FrameId: frameId},
		})
		msg = dapRecv(t, conn, reader)
		scopesResp, ok := msg.(*dap.ScopesResponse)
		require.True(t, ok, "expected ScopesResponse, got %T", msg)
		require.NotEmpty(t, scopesResp.Body.Scopes)
		localsRef := scopesResp.Body.Scopes[0].VariablesReference
		assert.Greater(t, localsRef, 0, "Locals scope should have a variable reference")

		dapSend(t, writer, &dap.VariablesRequest{
			Request: dap.Request{
				ProtocolMessage: dap.ProtocolMessage{Seq: nextSeq(), Type: "request"},
				Command:         "variables",
			},
			Arguments: dap.VariablesArguments{VariablesReference: localsRef},
		})
		msg = dapRecv(t, conn, reader)
		varsResp, ok := msg.(*dap.VariablesResponse)
		require.True(t, ok, "expected VariablesResponse, got %T", msg)
		require.NotEmpty(t, varsResp.Body.Variables)

		varByName := make(map[string]dap.Variable)
		for _, v := range varsResp.Body.Variables {
			varByName[v.Name] = v
		}

		if v, ok := varByName["intVar"]; ok {
			assert.Equal(t, "42", v.Value)
			assert.Equal(t, "int", v.Type)
		} else {
			t.Error("expected DAP variable intVar")
		}

		if v, ok := varByName["floatVar"]; ok {
			assert.Equal(t, "float", v.Type)
			assert.Contains(t, v.Value, "3.14")
		} else {
			t.Error("expected DAP variable floatVar")
		}

		if v, ok := varByName["stringVar"]; ok {
			assert.Equal(t, "string", v.Type)
			assert.Equal(t, `"hello"`, v.Value)
		} else {
			t.Error("expected DAP variable stringVar")
		}

		if v, ok := varByName["boolVar"]; ok {
			assert.Equal(t, "true", v.Value)
		} else {
			t.Error("expected DAP variable boolVar")
		}

		if v, ok := varByName["nullVar"]; ok {
			assert.Equal(t, "null", v.Type)
		} else {
			t.Error("expected DAP variable nullVar")
		}

		if v, ok := varByName["arr"]; ok {
			assert.Equal(t, "array", v.Type)
			assert.Contains(t, v.Value, "array(3)")
			assert.Greater(t, v.VariablesReference, 0, "$arr should be expandable")

			dapSend(t, writer, &dap.VariablesRequest{
				Request: dap.Request{
					ProtocolMessage: dap.ProtocolMessage{Seq: nextSeq(), Type: "request"},
					Command:         "variables",
				},
				Arguments: dap.VariablesArguments{VariablesReference: v.VariablesReference},
			})
			msg = dapRecv(t, conn, reader)
			arrResp, ok := msg.(*dap.VariablesResponse)
			require.True(t, ok, "expected VariablesResponse for arr expansion, got %T", msg)
			assert.Len(t, arrResp.Body.Variables, 3)
			if len(arrResp.Body.Variables) >= 3 {
				assert.Equal(t, "[0]", arrResp.Body.Variables[0].Name)
			}
		} else {
			t.Error("expected DAP variable arr")
		}

		if v, ok := varByName["fh"]; ok {
			assert.Equal(t, "resource", v.Type)
			assert.Contains(t, v.Value, "resource #")
		} else {
			t.Error("expected DAP variable fh")
		}

		dapSend(t, writer, &dap.ContinueRequest{
			Request: dap.Request{
				ProtocolMessage: dap.ProtocolMessage{Seq: nextSeq(), Type: "request"},
				Command:         "continue",
			},
			Arguments: dap.ContinueArguments{ThreadId: threadId},
		})
		msg = dapRecv(t, conn, reader)
		_, ok = msg.(*dap.ContinueResponse)
		require.True(t, ok, "expected ContinueResponse, got %T", msg)

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for request to complete")
		}

		dapSend(t, writer, &dap.DisconnectRequest{
			Request: dap.Request{
				ProtocolMessage: dap.ProtocolMessage{Seq: nextSeq(), Type: "request"},
				Command:         "disconnect",
			},
		})
		msg = dapRecv(t, conn, reader)
		_, ok = msg.(*dap.DisconnectResponse)
		assert.True(t, ok, "expected DisconnectResponse, got %T", msg)

		frankenphp.ClearBreakpoints()
	}, opts)
}

func TestDAPVariableFormatting(t *testing.T) {
	opts := debuggerTestOpts()
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		status := frankenphp.DebuggerStatus()
		require.NotEmpty(t, status.DAPListen)

		conn, err := net.DialTimeout("tcp", status.DAPListen, 2*time.Second)
		require.NoError(t, err)
		defer conn.Close()

		writer := bufio.NewWriter(conn)
		reader := bufio.NewReader(conn)
		seq := 0
		nextSeq := func() int { seq++; return seq }

		dapSend(t, writer, &dap.InitializeRequest{
			Request: dap.Request{
				ProtocolMessage: dap.ProtocolMessage{Seq: nextSeq(), Type: "request"},
				Command:         "initialize",
			},
			Arguments: dap.InitializeRequestArguments{AdapterID: "test"},
		})
		dapRecv(t, conn, reader) // InitializeResponse
		dapRecv(t, conn, reader) // InitializedEvent

		cwd, _ := os.Getwd()
		bpFile := filepath.Join(cwd, "testdata", "debugger-objects.php")
		dapSend(t, writer, &dap.SetBreakpointsRequest{
			Request: dap.Request{
				ProtocolMessage: dap.ProtocolMessage{Seq: nextSeq(), Type: "request"},
				Command:         "setBreakpoints",
			},
			Arguments: dap.SetBreakpointsArguments{
				Source:      dap.Source{Path: bpFile},
				Breakpoints: []dap.SourceBreakpoint{{Line: 11}},
			},
		})
		dapRecv(t, conn, reader)

		dapSend(t, writer, &dap.ConfigurationDoneRequest{
			Request: dap.Request{
				ProtocolMessage: dap.ProtocolMessage{Seq: nextSeq(), Type: "request"},
				Command:         "configurationDone",
			},
		})
		dapRecv(t, conn, reader)

		done := make(chan string, 1)
		go func() {
			body, _ := testGet("http://example.com/debugger-objects.php", handler, t)
			done <- body
		}()

		msg := dapRecv(t, conn, reader)
		stopped, ok := msg.(*dap.StoppedEvent)
		require.True(t, ok, "expected StoppedEvent, got %T", msg)
		threadId := stopped.Body.ThreadId

		dapSend(t, writer, &dap.StackTraceRequest{
			Request: dap.Request{
				ProtocolMessage: dap.ProtocolMessage{Seq: nextSeq(), Type: "request"},
				Command:         "stackTrace",
			},
			Arguments: dap.StackTraceArguments{ThreadId: threadId},
		})
		msg = dapRecv(t, conn, reader)
		stackResp := msg.(*dap.StackTraceResponse)
		frameId := stackResp.Body.StackFrames[0].Id

		dapSend(t, writer, &dap.ScopesRequest{
			Request: dap.Request{
				ProtocolMessage: dap.ProtocolMessage{Seq: nextSeq(), Type: "request"},
				Command:         "scopes",
			},
			Arguments: dap.ScopesArguments{FrameId: frameId},
		})
		msg = dapRecv(t, conn, reader)
		scopesResp := msg.(*dap.ScopesResponse)
		localsRef := scopesResp.Body.Scopes[0].VariablesReference

		dapSend(t, writer, &dap.VariablesRequest{
			Request: dap.Request{
				ProtocolMessage: dap.ProtocolMessage{Seq: nextSeq(), Type: "request"},
				Command:         "variables",
			},
			Arguments: dap.VariablesArguments{VariablesReference: localsRef},
		})
		msg = dapRecv(t, conn, reader)
		varsResp := msg.(*dap.VariablesResponse)

		varByName := make(map[string]dap.Variable)
		for _, v := range varsResp.Body.Variables {
			varByName[v.Name] = v
		}

		// Object should show class name as value and be expandable
		if v, ok := varByName["pt"]; ok {
			assert.Equal(t, "object", v.Type)
			assert.Equal(t, "Point", v.Value)
			assert.Greater(t, v.VariablesReference, 0, "object should be expandable")

			dapSend(t, writer, &dap.VariablesRequest{
				Request: dap.Request{
					ProtocolMessage: dap.ProtocolMessage{Seq: nextSeq(), Type: "request"},
					Command:         "variables",
				},
				Arguments: dap.VariablesArguments{VariablesReference: v.VariablesReference},
			})
			msg = dapRecv(t, conn, reader)
			propsResp, ok := msg.(*dap.VariablesResponse)
			require.True(t, ok)
			propByName := make(map[string]dap.Variable)
			for _, p := range propsResp.Body.Variables {
				propByName[p.Name] = p
			}
			if xp, ok := propByName["x"]; ok {
				assert.Equal(t, "int", xp.Type)
				assert.Equal(t, "10", xp.Value)
			} else {
				t.Errorf("expected property x, got properties: %v", propNames(propsResp.Body.Variables))
			}
			if yp, ok := propByName["y"]; ok {
				assert.Equal(t, "int", yp.Type)
				assert.Equal(t, "20", yp.Value)
			} else {
				t.Errorf("expected property y, got properties: %v", propNames(propsResp.Body.Variables))
			}
		} else {
			t.Errorf("expected DAP variable pt, got variables: %v", varNames(varsResp.Body.Variables))
		}

		dapSend(t, writer, &dap.ContinueRequest{
			Request: dap.Request{
				ProtocolMessage: dap.ProtocolMessage{Seq: nextSeq(), Type: "request"},
				Command:         "continue",
			},
			Arguments: dap.ContinueArguments{ThreadId: threadId},
		})
		dapRecv(t, conn, reader) // ContinueResponse

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for request to complete")
		}

		dapSend(t, writer, &dap.DisconnectRequest{
			Request: dap.Request{
				ProtocolMessage: dap.ProtocolMessage{Seq: nextSeq(), Type: "request"},
				Command:         "disconnect",
			},
		})
		dapRecv(t, conn, reader)
		frankenphp.ClearBreakpoints()
	}, opts)
}

func propNames(vars []dap.Variable) []string {
	names := make([]string, len(vars))
	for i, v := range vars {
		names[i] = fmt.Sprintf("%s=%s", v.Name, v.Value)
	}
	return names
}

func varNames(vars []dap.Variable) []string {
	names := make([]string, len(vars))
	for i, v := range vars {
		names[i] = v.Name
	}
	return names
}
