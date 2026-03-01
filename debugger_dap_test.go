package frankenphp_test

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
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
