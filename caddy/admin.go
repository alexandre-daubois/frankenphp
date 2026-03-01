package caddy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/caddyserver/caddy/v2"
	"github.com/dunglas/frankenphp"
)

type FrankenPHPAdmin struct {
}

// if the id starts with "admin.api" the module will register AdminRoutes via module.Routes()
func (FrankenPHPAdmin) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "admin.api.frankenphp",
		New: func() caddy.Module { return new(FrankenPHPAdmin) },
	}
}

// EXPERIMENTAL: These routes are not yet stable and may change in the future.
func (admin FrankenPHPAdmin) Routes() []caddy.AdminRoute {
	return []caddy.AdminRoute{
		{
			Pattern: "/frankenphp/workers/restart",
			Handler: caddy.AdminHandlerFunc(admin.restartWorkers),
		},
		{
			Pattern: "/frankenphp/threads",
			Handler: caddy.AdminHandlerFunc(admin.threads),
		},
		{
			Pattern: "/frankenphp/debug/status",
			Handler: caddy.AdminHandlerFunc(admin.debugStatus),
		},
		{
			Pattern: "/frankenphp/debug/breakpoints",
			Handler: caddy.AdminHandlerFunc(admin.debugBreakpoints),
		},
		{
			Pattern: "/frankenphp/debug/threads",
			Handler: caddy.AdminHandlerFunc(admin.debugThreads),
		},
		{
			Pattern: "/frankenphp/debug/continue",
			Handler: caddy.AdminHandlerFunc(admin.debugContinue),
		},
	}
}

func (admin *FrankenPHPAdmin) restartWorkers(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return admin.error(http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
	}

	frankenphp.RestartWorkers()
	caddy.Log().Info("workers restarted from admin api")
	admin.success(w, "workers restarted successfully\n")

	return nil
}

func (admin *FrankenPHPAdmin) threads(w http.ResponseWriter, _ *http.Request) error {
	debugState := frankenphp.DebugState()
	prettyJson, err := json.MarshalIndent(debugState, "", "    ")
	if err != nil {
		return admin.error(http.StatusInternalServerError, err)
	}

	return admin.success(w, string(prettyJson))
}

func (admin *FrankenPHPAdmin) debugStatus(w http.ResponseWriter, _ *http.Request) error {
	return admin.json(w, frankenphp.DebuggerStatus())
}

func (admin *FrankenPHPAdmin) debugBreakpoints(w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case http.MethodGet:
		bps := frankenphp.ListBreakpoints()
		return admin.json(w, bps)

	case http.MethodPost:
		var req struct {
			File string `json:"file"`
			Line int    `json:"line"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return admin.error(http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
		}
		if req.File == "" || req.Line <= 0 {
			return admin.error(http.StatusBadRequest, fmt.Errorf("file and line (>0) are required"))
		}
		id := frankenphp.SetBreakpoint(req.File, req.Line)
		return admin.json(w, frankenphp.BreakpointInfo{ID: id, File: req.File, Line: req.Line})

	case http.MethodDelete:
		idStr := r.URL.Query().Get("id")
		if idStr == "" {
			frankenphp.ClearBreakpoints()
			return admin.success(w, "all breakpoints cleared\n")
		}
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return admin.error(http.StatusBadRequest, fmt.Errorf("invalid breakpoint id: %w", err))
		}
		if !frankenphp.RemoveBreakpoint(id) {
			return admin.error(http.StatusNotFound, fmt.Errorf("breakpoint %d not found", id))
		}
		return admin.success(w, fmt.Sprintf("breakpoint %d removed\n", id))

	default:
		return admin.error(http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
	}
}

func (admin *FrankenPHPAdmin) debugThreads(w http.ResponseWriter, _ *http.Request) error {
	debugState := frankenphp.DebugState()
	var paused []frankenphp.ThreadDebugState
	for _, t := range debugState.ThreadDebugStates {
		if t.IsDebugPaused {
			paused = append(paused, t)
		}
	}
	return admin.json(w, paused)
}

func (admin *FrankenPHPAdmin) debugContinue(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return admin.error(http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
	}

	var req struct {
		Thread int    `json:"thread"`
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return admin.error(http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
	}

	switch strings.ToLower(req.Action) {
	case "continue", "":
		frankenphp.ContinueThread(req.Thread)
	case "step_over":
		frankenphp.StepOver(req.Thread)
	case "step_into":
		frankenphp.StepInto(req.Thread)
	case "step_out":
		frankenphp.StepOut(req.Thread)
	default:
		return admin.error(http.StatusBadRequest, fmt.Errorf("unknown action %q, expected: continue, step_over, step_into, step_out", req.Action))
	}

	return admin.success(w, fmt.Sprintf("thread %d: %s\n", req.Thread, req.Action))
}

func (admin *FrankenPHPAdmin) json(w http.ResponseWriter, v any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	return json.NewEncoder(w).Encode(v)
}

func (admin *FrankenPHPAdmin) success(w http.ResponseWriter, message string) error {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(message))
	return err
}

func (admin *FrankenPHPAdmin) error(statusCode int, err error) error {
	return caddy.APIError{HTTPStatus: statusCode, Err: err}
}
