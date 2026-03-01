#ifndef FRANKENPHP_DEBUGGER_H
#define FRANKENPHP_DEBUGGER_H

#include <pthread.h>
#include <stdbool.h>
#include <stdatomic.h>
#include <stdint.h>

#include "php.h"
#include "zend_compile.h"
#include "zend_execute.h"

extern _Atomic bool frankenphp_debugger_enabled;
extern _Atomic int frankenphp_debugger_exception_mode;

typedef enum {
	DBG_RUNNING,
	DBG_STEP_OVER,
	DBG_STEP_INTO,
	DBG_STEP_OUT,
	DBG_PAUSED,
} frankenphp_debug_mode_t;

extern __thread frankenphp_debug_mode_t dbg_mode;
extern __thread int dbg_step_depth;

typedef struct {
	const char *name;
	uint8_t type;
	zval *value;
} frankenphp_debug_variable_t;

typedef struct {
	const char *filename;
	const char *function_name;
	const char *class_name;
	uint32_t lineno;
	int num_vars;
	frankenphp_debug_variable_t *vars;
} frankenphp_debug_frame_t;

typedef struct {
	char *filename;
	uint32_t lineno;
	int id;
	bool enabled;
	char *condition;
	char *hit_condition;
	char *log_message;
	uint32_t hit_count;
} frankenphp_breakpoint_t;

frankenphp_debug_frame_t *frankenphp_debugger_get_captured_frames(int *out_depth);
frankenphp_debug_variable_t *frankenphp_debugger_get_captured_locals(int *out_count);
frankenphp_debug_variable_t *frankenphp_debugger_get_captured_globals(int *out_count);

void frankenphp_debugger_init(void);
void frankenphp_debugger_init_thread(int idx);
void frankenphp_debugger_shutdown(void);

void frankenphp_debugger_pause(const char *filename, uint32_t lineno);
void frankenphp_debugger_pause_exception(const char *filename, uint32_t lineno,
                                          const char *exception_class,
                                          const char *exception_message);

// cmd: 0=continue, 1=step_over, 2=step_into, 3=step_out
void frankenphp_debugger_resume_thread(int thread_idx, int cmd);

int frankenphp_debugger_add_breakpoint(const char *filename, uint32_t lineno,
                                        const char *condition,
                                        const char *hit_condition,
                                        const char *log_message);
bool frankenphp_debugger_remove_breakpoint(int breakpoint_id);
void frankenphp_debugger_clear_breakpoints(void);

int frankenphp_debugger_get_stack_depth(void);
frankenphp_debug_frame_t *frankenphp_debugger_get_stack(int *out_depth);
void frankenphp_debugger_free_stack(frankenphp_debug_frame_t *frames, int depth);
frankenphp_debug_variable_t *frankenphp_debugger_get_locals(int *out_count);
void frankenphp_debugger_free_locals(frankenphp_debug_variable_t *vars, int count);
frankenphp_debug_variable_t *frankenphp_debugger_get_globals(int *out_count);
void frankenphp_debugger_free_globals(frankenphp_debug_variable_t *vars, int count);

const char *frankenphp_debugger_object_class_name(zval *obj);
frankenphp_debug_variable_t *frankenphp_debugger_object_vars(zval *obj, int *out_count);
frankenphp_debug_variable_t *frankenphp_debugger_array_vars(zval *arr, int *out_count);
void frankenphp_debugger_free_array_vars(frankenphp_debug_variable_t *vars, int count);
const char *frankenphp_debugger_resource_type(zval *res);
int frankenphp_debugger_resource_id(zval *res);

#endif
