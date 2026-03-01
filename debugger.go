package frankenphp

/*
#include "debugger.h"
*/
import "C"
import (
	"path/filepath"
	"sync"
	"unsafe"

	"github.com/dunglas/frankenphp/internal/state"
)

type debugCommand int

const (
	debugContinue debugCommand = iota
	debugStepOver
	debugStepInto
	debugStepOut
)

type DebugBreakpointHit struct {
	ThreadIndex int
	File        string
	Line        int
}

type DebugFrame struct {
	File      string
	Function  string
	Class     string
	Line      int
	Variables []DebugVariable
}

type DebugVariable struct {
	Name  string
	Type  string
	Value any
}

type BreakpointInfo struct {
	ID   int    `json:"id"`
	File string `json:"file"`
	Line int    `json:"line"`
}

var (
	debugMu          sync.RWMutex
	debugListeners   []chan DebugBreakpointHit
	debugThreadInfo  map[int]*threadDebugInfo
	debugBreakpoints map[int]BreakpointInfo
)

type threadDebugInfo struct {
	File   string
	Line   int
	Stack  []DebugFrame
	Locals []DebugVariable
}

type DebuggerStatusInfo struct {
	Enabled     bool             `json:"enabled"`
	Breakpoints []BreakpointInfo `json:"breakpoints"`
	DAPListen   string           `json:"dapListen"`
}

var debugDAPListen string

func DebuggerStatus() DebuggerStatusInfo {
	return DebuggerStatusInfo{
		Enabled:     bool(C.frankenphp_debugger_enabled),
		Breakpoints: ListBreakpoints(),
		DAPListen:   debugDAPListen,
	}
}

func initDebugger() {
	debugMu.Lock()
	defer debugMu.Unlock()
	debugThreadInfo = make(map[int]*threadDebugInfo)
	debugBreakpoints = make(map[int]BreakpointInfo)
}

func shutdownDebugger() {
	debugMu.Lock()
	defer debugMu.Unlock()
	// Resume all paused threads
	for idx := range debugThreadInfo {
		C.frankenphp_debugger_resume_thread(C.int(idx), 0)
	}
	for _, l := range debugListeners {
		close(l)
	}
	debugThreadInfo = nil
	debugListeners = nil
}

// go_debugger_notify_breakpoint is called from C when a breakpoint is hit.
// It returns immediately — the C thread blocks on pthread_cond_wait instead.
//
//export go_debugger_notify_breakpoint
func go_debugger_notify_breakpoint(threadIndex C.uintptr_t, filename *C.char, lineno C.uint32_t) {
	idx := int(threadIndex)
	file := C.GoString(filename)
	line := int(lineno)

	stack := readCapturedStack()
	locals := readCapturedLocals()

	debugMu.Lock()

	debugThreadInfo[idx] = &threadDebugInfo{File: file, Line: line, Stack: stack, Locals: locals}

	thread := phpThreads[idx]
	thread.state.Set(state.DebugPaused)

	hit := DebugBreakpointHit{ThreadIndex: idx, File: file, Line: line}
	for _, l := range debugListeners {
		select {
		case l <- hit:
		default:
		}
	}
	debugMu.Unlock()
}

// resumeThread wakes the C thread and cleans up Go-side state
func resumeThread(threadIndex int, cmd debugCommand) {
	debugMu.Lock()
	delete(debugThreadInfo, threadIndex)
	thread := phpThreads[threadIndex]
	debugMu.Unlock()

	C.frankenphp_debugger_resume_thread(C.int(threadIndex), C.int(cmd))

	thread.state.Set(state.Ready)
}

func SetBreakpoint(filename string, line int) int {
	if resolved, err := filepath.EvalSymlinks(filename); err == nil {
		if abs, err := filepath.Abs(resolved); err == nil {
			filename = abs
		}
	}

	cFilename := C.CString(filename)
	defer C.free(unsafe.Pointer(cFilename))
	id := int(C.frankenphp_debugger_add_breakpoint(cFilename, C.uint32_t(line)))

	debugMu.Lock()
	debugBreakpoints[id] = BreakpointInfo{ID: id, File: filename, Line: line}
	debugMu.Unlock()

	return id
}

func RemoveBreakpoint(id int) bool {
	ok := bool(C.frankenphp_debugger_remove_breakpoint(C.int(id)))
	if ok {
		debugMu.Lock()
		delete(debugBreakpoints, id)
		debugMu.Unlock()
	}
	return ok
}

func ClearBreakpoints() {
	C.frankenphp_debugger_clear_breakpoints()
	debugMu.Lock()
	debugBreakpoints = make(map[int]BreakpointInfo)
	debugMu.Unlock()
}

func ListBreakpoints() []BreakpointInfo {
	debugMu.RLock()
	defer debugMu.RUnlock()
	bps := make([]BreakpointInfo, 0, len(debugBreakpoints))
	for _, bp := range debugBreakpoints {
		bps = append(bps, bp)
	}
	return bps
}

func ContinueThread(threadIndex int) {
	resumeThread(threadIndex, debugContinue)
}

func StepOver(threadIndex int) {
	resumeThread(threadIndex, debugStepOver)
}

func StepInto(threadIndex int) {
	resumeThread(threadIndex, debugStepInto)
}

func StepOut(threadIndex int) {
	resumeThread(threadIndex, debugStepOut)
}

func GetStackTrace(threadIndex int) []DebugFrame {
	debugMu.RLock()
	defer debugMu.RUnlock()
	if info, ok := debugThreadInfo[threadIndex]; ok {
		return info.Stack
	}
	return nil
}

func GetLocals(threadIndex int) []DebugVariable {
	debugMu.RLock()
	defer debugMu.RUnlock()
	if info, ok := debugThreadInfo[threadIndex]; ok {
		return info.Locals
	}
	return nil
}

// readCapturedStack reads frames captured by C in pause_thread.
// Uses C accessor function since Go can't read __thread variables directly.
// MUST be called from the CGo callback (same OS thread as the PHP thread).
func readCapturedStack() []DebugFrame {
	var cDepth C.int
	cFrames := C.frankenphp_debugger_get_captured_frames(&cDepth)
	depth := int(cDepth)
	if depth == 0 || cFrames == nil {
		return nil
	}

	cSlice := unsafe.Slice(cFrames, depth)
	frames := make([]DebugFrame, depth)
	for i := 0; i < depth; i++ {
		f := cSlice[i]
		frame := DebugFrame{
			File:     C.GoString(f.filename),
			Function: C.GoString(f.function_name),
			Line:     int(f.lineno),
		}
		if f.class_name != nil {
			frame.Class = C.GoString(f.class_name)
		}

		nv := int(f.num_vars)
		if nv > 0 && f.vars != nil {
			vars := unsafe.Slice(f.vars, nv)
			frame.Variables = make([]DebugVariable, nv)
			for j := 0; j < nv; j++ {
				v := vars[j]
				frame.Variables[j] = DebugVariable{
					Name: C.GoString(v.name),
					Type: zvalTypeName(int(v._type)),
				}
				if v.value != nil {
					frame.Variables[j].Value = zvalToGo(v.value)
				}
			}
		}
		frames[i] = frame
	}
	return frames
}

// readCapturedLocals reads locals captured by C in pause_thread.
// Uses C accessor function since Go can't read __thread variables directly.
func readCapturedLocals() []DebugVariable {
	var cCount C.int
	cVars := C.frankenphp_debugger_get_captured_locals(&cCount)
	count := int(cCount)
	if count == 0 || cVars == nil {
		return nil
	}

	cSlice := unsafe.Slice(cVars, count)
	vars := make([]DebugVariable, count)
	for i := 0; i < count; i++ {
		v := cSlice[i]
		vars[i] = DebugVariable{
			Name: C.GoString(v.name),
			Type: zvalTypeName(int(v._type)),
		}
		if v.value != nil {
			vars[i].Value = zvalToGo(v.value)
		}
	}
	return vars
}

func SubscribeBreakpoints() <-chan DebugBreakpointHit {
	ch := make(chan DebugBreakpointHit, 16)
	debugMu.Lock()
	debugListeners = append(debugListeners, ch)
	debugMu.Unlock()
	return ch
}

func getThreadDebugInfo(threadIndex int) threadDebugInfo {
	debugMu.RLock()
	defer debugMu.RUnlock()
	if info, ok := debugThreadInfo[threadIndex]; ok {
		return *info
	}
	return threadDebugInfo{}
}

func zvalTypeName(t int) string {
	switch t {
	case 0:
		return "undef"
	case 1:
		return "null"
	case 2:
		return "false"
	case 3:
		return "true"
	case 4:
		return "int"
	case 5:
		return "float"
	case 6:
		return "string"
	case 7:
		return "array"
	case 8:
		return "object"
	case 9:
		return "resource"
	default:
		return "unknown"
	}
}

func zvalToGo(zval *C.zval) any {
	val, err := GoValue[any](unsafe.Pointer(zval))
	if err != nil {
		return nil
	}
	return val
}
