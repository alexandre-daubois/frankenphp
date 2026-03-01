package caddy_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dunglas/frankenphp/internal/fastabs"

	"github.com/caddyserver/caddy/v2/caddytest"
	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
)

func TestRestartWorkerViaAdminApi(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`

			frankenphp {
				worker ../testdata/worker-with-counter.php 1
			}
		}

		localhost:`+testPort+` {
			route {
				root ../testdata
				rewrite worker-with-counter.php
				php
			}
		}
		`, "caddyfile")

	tester.AssertGetResponse("http://localhost:"+testPort+"/", http.StatusOK, "requests:1")
	tester.AssertGetResponse("http://localhost:"+testPort+"/", http.StatusOK, "requests:2")

	assertAdminResponse(t, tester, "POST", "workers/restart", http.StatusOK, "workers restarted successfully\n")

	tester.AssertGetResponse("http://localhost:"+testPort+"/", http.StatusOK, "requests:1")
}

func TestShowTheCorrectThreadDebugStatus(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`

			frankenphp {
				num_threads 3
				max_threads 6
				worker ../testdata/worker-with-counter.php 1
				worker ../testdata/index.php 1
			}
		}

		localhost:`+testPort+` {
			route {
				root ../testdata
				rewrite worker-with-counter.php
				php
			}
		}
		`, "caddyfile")

	debugState := getDebugState(t, tester)

	// assert that the correct threads are present in the thread info
	assert.Equal(t, debugState.ThreadDebugStates[0].State, "ready")
	assert.Contains(t, debugState.ThreadDebugStates[1].Name, "worker-with-counter.php")
	assert.Contains(t, debugState.ThreadDebugStates[2].Name, "index.php")
	assert.Equal(t, debugState.ReservedThreadCount, 3)
	assert.Len(t, debugState.ThreadDebugStates, 3)
}

func TestAutoScaleWorkerThreads(t *testing.T) {
	wg := sync.WaitGroup{}
	maxTries := 10
	requestsPerTry := 200
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`

			frankenphp {
				max_threads 10
				num_threads 2
				worker ../testdata/sleep.php {
					num 1
					max_threads 3
				}
			}
		}

		localhost:`+testPort+` {
			route {
				root ../testdata
				rewrite sleep.php
				php
			}
		}
		`, "caddyfile")

	// spam an endpoint that simulates IO
	endpoint := "http://localhost:" + testPort + "/?sleep=2&work=1000"
	amountOfThreads := getNumThreads(t, tester)

	// try to spawn the additional threads by spamming the server
	for range maxTries {
		wg.Add(requestsPerTry)
		for range requestsPerTry {
			go func() {
				tester.AssertGetResponse(endpoint, http.StatusOK, "slept for 2 ms and worked for 1000 iterations")
				wg.Done()
			}()
		}
		wg.Wait()

		amountOfThreads = getNumThreads(t, tester)
		if amountOfThreads > 2 {
			break
		}
	}

	assert.NotEqual(t, amountOfThreads, 2, "at least one thread should have been auto-scaled")
	assert.LessOrEqual(t, amountOfThreads, 4, "at most 3 max_threads + 1 regular thread should be present")
}

// Note this test requires at least 2x40MB available memory for the process
func TestAutoScaleRegularThreadsOnAutomaticThreadLimit(t *testing.T) {
	wg := sync.WaitGroup{}
	maxTries := 10
	requestsPerTry := 200
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`

			frankenphp {
				max_threads auto
				num_threads 1
				php_ini memory_limit 40M # a reasonable limit for the test
			}
		}

		localhost:`+testPort+` {
			route {
				root ../testdata
				php
			}
		}
		`, "caddyfile")

	// spam an endpoint that simulates IO
	endpoint := "http://localhost:" + testPort + "/sleep.php?sleep=2&work=1000"
	amountOfThreads := getNumThreads(t, tester)

	// try to spawn the additional threads by spamming the server
	for range maxTries {
		wg.Add(requestsPerTry)
		for range requestsPerTry {
			go func() {
				tester.AssertGetResponse(endpoint, http.StatusOK, "slept for 2 ms and worked for 1000 iterations")
				wg.Done()
			}()
		}
		wg.Wait()

		amountOfThreads = getNumThreads(t, tester)
		if amountOfThreads > 1 {
			break
		}
	}

	// assert that there are now more threads present
	assert.NotEqual(t, amountOfThreads, 1)
}

func assertAdminResponse(t *testing.T, tester *caddytest.Tester, method string, path string, expectedStatus int, expectedBody string) {
	adminUrl := "http://localhost:2999/frankenphp/"
	r, err := http.NewRequest(method, adminUrl+path, nil)
	assert.NoError(t, err)
	if expectedBody == "" {
		_ = tester.AssertResponseCode(r, expectedStatus)
		return
	}
	_, _ = tester.AssertResponse(r, expectedStatus, expectedBody)
}

func getAdminResponseBody(t *testing.T, tester *caddytest.Tester, method string, path string) string {
	adminUrl := "http://localhost:2999/frankenphp/"
	r, err := http.NewRequest(method, adminUrl+path, nil)
	assert.NoError(t, err)
	resp := tester.AssertResponseCode(r, http.StatusOK)
	defer resp.Body.Close()
	bytes, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	return string(bytes)
}

func getDebugState(t *testing.T, tester *caddytest.Tester) frankenphp.FrankenPHPDebugState {
	t.Helper()
	threadStates := getAdminResponseBody(t, tester, "GET", "threads")

	var debugStates frankenphp.FrankenPHPDebugState
	err := json.Unmarshal([]byte(threadStates), &debugStates)
	assert.NoError(t, err)

	return debugStates
}

func getNumThreads(t *testing.T, tester *caddytest.Tester) int {
	t.Helper()
	return len(getDebugState(t, tester).ThreadDebugStates)
}

func TestAddModuleWorkerViaAdminApi(t *testing.T) {
	// Initialize a server with admin API enabled
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`
		}

		localhost:`+testPort+` {
			route {
				root ../testdata
				php
			}
		}
		`, "caddyfile")

	// Get initial debug state to check number of workers
	initialDebugState := getDebugState(t, tester)
	initialWorkerCount := 0
	for _, thread := range initialDebugState.ThreadDebugStates {
		if strings.HasPrefix(thread.Name, "Worker PHP Thread") {
			initialWorkerCount++
		}
	}

	// Create a Caddyfile configuration with a module worker
	workerConfig := `
	{
		skip_install_trust
		admin localhost:2999
		http_port ` + testPort + `
	}

	localhost:` + testPort + ` {
		route {
			root ../testdata
			php {
				worker ../testdata/worker-with-counter.php 1
			}
		}
	}
	`

	// Send the configuration to the admin API
	adminUrl := "http://localhost:2999/load"
	r, err := http.NewRequest("POST", adminUrl, bytes.NewBufferString(workerConfig))
	assert.NoError(t, err)
	r.Header.Set("Content-Type", "text/caddyfile")
	resp := tester.AssertResponseCode(r, http.StatusOK)
	defer resp.Body.Close()

	// Get the updated debug state to check if the worker was added
	updatedDebugState := getDebugState(t, tester)
	updatedWorkerCount := 0
	workerFound := false
	filename, _ := fastabs.FastAbs("../testdata/worker-with-counter.php")
	for _, thread := range updatedDebugState.ThreadDebugStates {
		if strings.HasPrefix(thread.Name, "Worker PHP Thread") {
			updatedWorkerCount++
			if thread.Name == "Worker PHP Thread - "+filename {
				workerFound = true
			}
		}
	}

	// Assert that the worker was added
	assert.Greater(t, updatedWorkerCount, initialWorkerCount, "Worker count should have increased")
	assert.True(t, workerFound, fmt.Sprintf("Worker with name %q should be found", "Worker PHP Thread - "+filename))

	// Make a request to the worker to verify it's working
	tester.AssertGetResponse("http://localhost:"+testPort+"/worker-with-counter.php", http.StatusOK, "requests:1")
}

func postAdminJSON(t *testing.T, tester *caddytest.Tester, path string, body string) *http.Response {
	t.Helper()
	adminUrl := "http://localhost:2999/frankenphp/"
	r, err := http.NewRequest("POST", adminUrl+path, bytes.NewBufferString(body))
	assert.NoError(t, err)
	r.Header.Set("Content-Type", "application/json")
	return tester.AssertResponseCode(r, http.StatusOK)
}

func deleteAdmin(t *testing.T, tester *caddytest.Tester, path string) *http.Response {
	t.Helper()
	adminUrl := "http://localhost:2999/frankenphp/"
	r, err := http.NewRequest("DELETE", adminUrl+path, nil)
	assert.NoError(t, err)
	return tester.AssertResponseCode(r, http.StatusOK)
}

func initDebugServer(t *testing.T) *caddytest.Tester {
	t.Helper()
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`

			frankenphp {
				debug {
					listen :0
				}
			}
		}

		localhost:`+testPort+` {
			route {
				root ../testdata
				php
			}
		}
		`, "caddyfile")
	return tester
}

func TestDebugAdminStatus(t *testing.T) {
	tester := initDebugServer(t)

	body := getAdminResponseBody(t, tester, "GET", "debug/status")
	var status frankenphp.DebuggerStatusInfo
	err := json.Unmarshal([]byte(body), &status)
	assert.NoError(t, err)
	assert.True(t, status.Enabled)
	assert.NotEmpty(t, status.DAPListen)
}

func TestDebugAdminBreakpointsCRUD(t *testing.T) {
	tester := initDebugServer(t)

	resp := postAdminJSON(t, tester, "debug/breakpoints", `{"file":"/test.php","line":10}`)
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var bp frankenphp.BreakpointInfo
	err := json.Unmarshal(respBody, &bp)
	assert.NoError(t, err)
	assert.Equal(t, 10, bp.Line)
	assert.NotZero(t, bp.ID)

	body := getAdminResponseBody(t, tester, "GET", "debug/breakpoints")
	var bps []frankenphp.BreakpointInfo
	err = json.Unmarshal([]byte(body), &bps)
	assert.NoError(t, err)
	assert.Len(t, bps, 1)

	deleteAdmin(t, tester, fmt.Sprintf("debug/breakpoints?id=%d", bp.ID))

	body = getAdminResponseBody(t, tester, "GET", "debug/breakpoints")
	err = json.Unmarshal([]byte(body), &bps)
	assert.NoError(t, err)
	assert.Empty(t, bps)

	postAdminJSON(t, tester, "debug/breakpoints", `{"file":"/a.php","line":1}`)
	postAdminJSON(t, tester, "debug/breakpoints", `{"file":"/b.php","line":2}`)
	deleteAdmin(t, tester, "debug/breakpoints")
	body = getAdminResponseBody(t, tester, "GET", "debug/breakpoints")
	err = json.Unmarshal([]byte(body), &bps)
	assert.NoError(t, err)
	assert.Empty(t, bps)
}

func TestDebugAdminBreakpointAndContinue(t *testing.T) {
	tester := initDebugServer(t)

	postAdminJSON(t, tester, "debug/breakpoints", `{"file":"../testdata/debugger-breakpoint.php","line":3}`)

	done := make(chan string, 1)
	go func() {
		resp, err := http.Get("http://localhost:" + testPort + "/debugger-breakpoint.php")
		if err != nil {
			done <- "error: " + err.Error()
			return
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		done <- string(body)
	}()

	var pausedThread int
	found := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		body := getAdminResponseBody(t, tester, "GET", "debug/threads")
		var threads []frankenphp.ThreadDebugState
		_ = json.Unmarshal([]byte(body), &threads)
		if len(threads) > 0 {
			pausedThread = threads[0].Index
			found = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	assert.True(t, found, "expected a paused thread within timeout")

	postAdminJSON(t, tester, "debug/continue", fmt.Sprintf(`{"thread":%d,"action":"continue"}`, pausedThread))

	select {
	case body := <-done:
		assert.Equal(t, "result=30", body)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for request to complete after continue")
	}
}
