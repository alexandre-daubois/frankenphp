#include "debugger.h"

#include <limits.h>
#include <stdlib.h>
#include <string.h>

#include "SAPI.h"
#include "zend_exceptions.h"
#include "zend_execute.h"

_Atomic bool frankenphp_debugger_enabled = false;

// 0=disabled, 1=uncaught only, 2=all
_Atomic int frankenphp_debugger_exception_mode = 0;

static void (*original_exception_hook)(zend_object *ex) = NULL;

static void (*original_execute_ex)(zend_execute_data *execute_data) = NULL;

static pthread_rwlock_t bp_lock = PTHREAD_RWLOCK_INITIALIZER;
static frankenphp_breakpoint_t *breakpoints = NULL;
static int breakpoint_count = 0;
static int breakpoint_capacity = 0;
static int next_breakpoint_id = 1;

__thread frankenphp_debug_mode_t dbg_mode = DBG_RUNNING;
__thread int dbg_step_depth = 0;
__thread uint32_t dbg_step_start_line = 0;
__thread const char *dbg_step_start_file = NULL;

__thread frankenphp_debug_frame_t *dbg_captured_frames = NULL;
__thread int dbg_captured_depth = 0;
__thread frankenphp_debug_variable_t *dbg_captured_locals = NULL;
__thread int dbg_captured_locals_count = 0;
__thread frankenphp_debug_variable_t *dbg_captured_globals = NULL;
__thread int dbg_captured_globals_count = 0;

extern __thread uintptr_t thread_index;

frankenphp_debug_frame_t *frankenphp_debugger_get_captured_frames(int *out_depth) {
	*out_depth = dbg_captured_depth;
	return dbg_captured_frames;
}

frankenphp_debug_variable_t *frankenphp_debugger_get_captured_locals(int *out_count) {
	*out_count = dbg_captured_locals_count;
	return dbg_captured_locals;
}

frankenphp_debug_variable_t *frankenphp_debugger_get_captured_globals(int *out_count) {
	*out_count = dbg_captured_globals_count;
	return dbg_captured_globals;
}

#include "_cgo_export.h"

#define MAX_DEBUG_THREADS 256
static pthread_mutex_t thread_pause_mu[MAX_DEBUG_THREADS];
static pthread_cond_t thread_pause_cond[MAX_DEBUG_THREADS];
static volatile int thread_pause_cmd[MAX_DEBUG_THREADS]; // -1 = paused/waiting, >= 0 = command

typedef struct {
	int id;
	bool should_break;
	char *log_output;
} bp_check_result_t;

static bool evaluate_php_condition(const char *expr) {
	zend_rebuild_symbol_table();
	int orig_no_ext = EG(no_extensions);
	EG(no_extensions) = 1;

	zval retval;
	ZVAL_UNDEF(&retval);
	zend_result res = zend_eval_string_ex((char *)expr, &retval,
	                                       "debugger condition", 1);
	EG(no_extensions) = orig_no_ext;

	if (res == FAILURE) {
		zval_ptr_dtor(&retval);
		return true;
	}

	bool result = zend_is_true(&retval);
	zval_ptr_dtor(&retval);
	return result;
}

static bool check_hit_condition(uint32_t hit_count, const char *cond) {
	if (!cond || cond[0] == '\0') {
		return true;
	}

	const char *p = cond;
	while (*p == ' ') p++;

	char op[4] = {0};
	int oi = 0;
	while (*p && !(*p >= '0' && *p <= '9') && oi < 3) {
		if (*p != ' ') {
			op[oi++] = *p;
		}
		p++;
	}
	while (*p == ' ') p++;

	unsigned long target = strtoul(p, NULL, 10);

	if (op[0] == '>' && op[1] == '=') return hit_count >= target;
	if (op[0] == '<' && op[1] == '=') return hit_count <= target;
	if (op[0] == '!' && op[1] == '=') return hit_count != target;
	if (op[0] == '=' && op[1] == '=') return hit_count == target;
	if (op[0] == '>')                 return hit_count > target;
	if (op[0] == '<')                 return hit_count < target;
	if (op[0] == '%')                 return target > 0 && (hit_count % target) == 0;

	return hit_count == target;
}

static char *format_log_message(const char *msg) {
	size_t len = strlen(msg);
	size_t cap = len * 2 + 1;
	char *out = malloc(cap);
	size_t oi = 0;

	for (size_t i = 0; i < len; i++) {
		if (msg[i] == '{') {
			int depth = 1;
			size_t end = i + 1;
			while (end < len && depth > 0) {
				if (msg[end] == '{') depth++;
				else if (msg[end] == '}') depth--;
				if (depth > 0) end++;
			}
			if (end < len) {
				size_t expr_len = end - i - 1;
				char *expr = malloc(expr_len + 1);
				memcpy(expr, msg + i + 1, expr_len);
				expr[expr_len] = '\0';

				zend_rebuild_symbol_table();
				int orig_no_ext = EG(no_extensions);
				EG(no_extensions) = 1;

				zval retval;
				ZVAL_UNDEF(&retval);
				zend_result res = zend_eval_string_ex(
				    expr, &retval, "debugger logpoint", 1);
				EG(no_extensions) = orig_no_ext;

				const char *val_str = NULL;
				zend_string *tmp_str = NULL;
				if (res == SUCCESS) {
					tmp_str = zval_get_string(&retval);
					val_str = ZSTR_VAL(tmp_str);
				}

				if (val_str) {
					size_t vl = strlen(val_str);
					while (oi + vl + 1 >= cap) {
						cap *= 2;
						out = realloc(out, cap);
					}
					memcpy(out + oi, val_str, vl);
					oi += vl;
				} else {
					size_t chunk = end - i + 1;
					while (oi + chunk + 1 >= cap) {
						cap *= 2;
						out = realloc(out, cap);
					}
					memcpy(out + oi, msg + i, chunk);
					oi += chunk;
				}

				if (tmp_str) zend_string_release(tmp_str);
				zval_ptr_dtor(&retval);
				free(expr);
				i = end;
				continue;
			}
		}

		if (oi + 2 >= cap) {
			cap *= 2;
			out = realloc(out, cap);
		}
		out[oi++] = msg[i];
	}

	out[oi] = '\0';
	return out;
}

static bp_check_result_t check_breakpoint_conditional(const char *filename,
                                                       uint32_t lineno) {
	bp_check_result_t result = {0, false, NULL};

	char *condition = NULL;
	char *hit_condition = NULL;
	char *log_message = NULL;
	uint32_t hit_count = 0;
	bool found = false;

	pthread_rwlock_wrlock(&bp_lock);
	for (int i = 0; i < breakpoint_count; i++) {
		if (breakpoints[i].enabled && breakpoints[i].lineno == lineno &&
		    strcmp(breakpoints[i].filename, filename) == 0) {
			breakpoints[i].hit_count++;
			hit_count = breakpoints[i].hit_count;
			result.id = breakpoints[i].id;
			if (breakpoints[i].condition)
				condition = strdup(breakpoints[i].condition);
			if (breakpoints[i].hit_condition)
				hit_condition = strdup(breakpoints[i].hit_condition);
			if (breakpoints[i].log_message)
				log_message = strdup(breakpoints[i].log_message);
			found = true;
			break;
		}
	}
	pthread_rwlock_unlock(&bp_lock);

	if (!found) {
		return result;
	}

	if (hit_condition && !check_hit_condition(hit_count, hit_condition)) {
		free(condition);
		free(hit_condition);
		free(log_message);
		return result;
	}

	if (condition && !evaluate_php_condition(condition)) {
		free(condition);
		free(hit_condition);
		free(log_message);
		return result;
	}

	if (log_message) {
		result.log_output = format_log_message(log_message);
	} else {
		result.should_break = true;
	}

	free(condition);
	free(hit_condition);
	free(log_message);
	return result;
}

void frankenphp_debugger_pause(const char *filename, uint32_t lineno) {
	int idx = (int)thread_index;
	dbg_mode = DBG_PAUSED;

#ifdef ZEND_MAX_EXECUTION_TIMERS
	zend_unset_timeout();
#endif

	if (dbg_captured_frames) {
		frankenphp_debugger_free_stack(dbg_captured_frames,
		                               dbg_captured_depth);
	}
	dbg_captured_frames = frankenphp_debugger_get_stack(&dbg_captured_depth);

	if (dbg_captured_locals) {
		frankenphp_debugger_free_locals(dbg_captured_locals,
		                                dbg_captured_locals_count);
	}
	dbg_captured_locals =
	    frankenphp_debugger_get_locals(&dbg_captured_locals_count);

	if (dbg_captured_globals) {
		frankenphp_debugger_free_globals(dbg_captured_globals,
		                                 dbg_captured_globals_count);
	}
	dbg_captured_globals =
	    frankenphp_debugger_get_globals(&dbg_captured_globals_count);

	thread_pause_cmd[idx] = -1;
	go_debugger_notify_breakpoint((GoUintptr)thread_index, (char *)filename,
	                              (GoUint32)lineno);

	// Block in C until Go calls frankenphp_debugger_resume_thread
	pthread_mutex_lock(&thread_pause_mu[idx]);
	while (thread_pause_cmd[idx] < 0) {
		pthread_cond_wait(&thread_pause_cond[idx],
		                  &thread_pause_mu[idx]);
	}
	int cmd = thread_pause_cmd[idx];
	thread_pause_cmd[idx] = -1;
	pthread_mutex_unlock(&thread_pause_mu[idx]);

	switch (cmd) {
	case 1:
		dbg_mode = DBG_STEP_OVER;
		dbg_step_depth = frankenphp_debugger_get_stack_depth();
		dbg_step_start_line = lineno;
		dbg_step_start_file = filename;
		break;
	case 2:
		dbg_mode = DBG_STEP_INTO;
		dbg_step_start_line = lineno;
		dbg_step_start_file = filename;
		break;
	case 3:
		dbg_mode = DBG_STEP_OUT;
		dbg_step_depth = frankenphp_debugger_get_stack_depth();
		dbg_step_start_line = 0;
		dbg_step_start_file = NULL;
		break;
	default:
		dbg_mode = DBG_RUNNING;
		dbg_step_start_line = 0;
		dbg_step_start_file = NULL;
		break;
	}

	if (dbg_captured_frames) {
		frankenphp_debugger_free_stack(dbg_captured_frames,
		                               dbg_captured_depth);
		dbg_captured_frames = NULL;
		dbg_captured_depth = 0;
	}
	if (dbg_captured_locals) {
		frankenphp_debugger_free_locals(dbg_captured_locals,
		                                dbg_captured_locals_count);
		dbg_captured_locals = NULL;
		dbg_captured_locals_count = 0;
	}
	if (dbg_captured_globals) {
		frankenphp_debugger_free_globals(dbg_captured_globals,
		                                 dbg_captured_globals_count);
		dbg_captured_globals = NULL;
		dbg_captured_globals_count = 0;
	}

#ifdef ZEND_MAX_EXECUTION_TIMERS
	if (PG(max_input_time) != -1) {
		zend_set_timeout(INI_INT("max_execution_time"), 0);
	}
#endif
}

static void frankenphp_debugger_execute_ex(zend_execute_data *execute_data) {
	if (!atomic_load_explicit(&frankenphp_debugger_enabled,
	                          memory_order_relaxed)) {
		original_execute_ex(execute_data);
		return;
	}

	if (execute_data->func && ZEND_USER_CODE(execute_data->func->type)) {
		const char *filename =
		    ZSTR_VAL(execute_data->func->op_array.filename);
		uint32_t lineno =
		    execute_data->opline ? execute_data->opline->lineno : 0;

		bool should_break = false;

		bp_check_result_t bp_result =
		    check_breakpoint_conditional(filename, lineno);
		if (bp_result.log_output) {
			go_debugger_notify_output(
			    (GoUintptr)thread_index,
			    bp_result.log_output);
			free(bp_result.log_output);
		} else if (bp_result.should_break) {
			should_break = true;
		}

		if (!should_break && dbg_mode != DBG_RUNNING) {
			bool same_line = (dbg_step_start_line == lineno &&
			                  dbg_step_start_file != NULL &&
			                  strcmp(dbg_step_start_file, filename) == 0);
			if (!same_line) {
				int current_depth =
				    frankenphp_debugger_get_stack_depth();
				switch (dbg_mode) {
				case DBG_STEP_INTO:
					should_break = true;
					break;
				case DBG_STEP_OVER:
					should_break =
					    (current_depth <= dbg_step_depth);
					break;
				case DBG_STEP_OUT:
					should_break =
					    (current_depth < dbg_step_depth);
					break;
				default:
					break;
				}
			}
		}

		if (should_break) {
			frankenphp_debugger_pause(filename, lineno);
		}
	}

	original_execute_ex(execute_data);
}

static int
frankenphp_debugger_ext_stmt_handler(zend_execute_data *execute_data) {
	if (!atomic_load_explicit(&frankenphp_debugger_enabled,
	                          memory_order_relaxed)) {
		return ZEND_USER_OPCODE_DISPATCH;
	}

	if (!execute_data->func || !ZEND_USER_CODE(execute_data->func->type)) {
		return ZEND_USER_OPCODE_DISPATCH;
	}

	const char *filename = ZSTR_VAL(execute_data->func->op_array.filename);
	uint32_t lineno = execute_data->opline->lineno;

	bool should_break = false;

	bp_check_result_t bp_result =
	    check_breakpoint_conditional(filename, lineno);
	if (bp_result.log_output) {
		go_debugger_notify_output(
		    (GoUintptr)thread_index,
		    bp_result.log_output);
		free(bp_result.log_output);
	} else if (bp_result.should_break) {
		should_break = true;
	}

	if (!should_break && dbg_mode != DBG_RUNNING) {
		bool same_line = (dbg_step_start_line == lineno &&
		                  dbg_step_start_file != NULL &&
		                  strcmp(dbg_step_start_file, filename) == 0);
		if (!same_line) {
			int depth = frankenphp_debugger_get_stack_depth();
			switch (dbg_mode) {
			case DBG_STEP_INTO:
				should_break = true;
				break;
			case DBG_STEP_OVER:
				should_break = (depth <= dbg_step_depth);
				break;
			case DBG_STEP_OUT:
				should_break = (depth < dbg_step_depth);
				break;
			default:
				break;
			}
		}
	}

	if (should_break) {
		frankenphp_debugger_pause(filename, lineno);
	}

	return ZEND_USER_OPCODE_DISPATCH;
}

static bool frankenphp_debugger_is_exception_caught(void) {
	zend_execute_data *ex = EG(current_execute_data);
	while (ex) {
		if (ex->func && ZEND_USER_CODE(ex->func->type) &&
		    ex->opline) {
			zend_op_array *op = &ex->func->op_array;
			uint32_t op_num =
			    (uint32_t)(ex->opline - op->opcodes);
			for (int i = 0; i < op->last_try_catch; i++) {
				zend_try_catch_element *tc =
				    &op->try_catch_array[i];
				if (op_num >= tc->try_op &&
				    (tc->catch_op > 0 &&
				     op_num < tc->catch_op)) {
					return true;
				}
				if (op_num >= tc->try_op &&
				    (tc->finally_op > 0 &&
				     op_num < tc->finally_op)) {
					return true;
				}
			}
		}
		ex = ex->prev_execute_data;
	}
	return false;
}

static void frankenphp_debugger_exception_hook(zend_object *ex) {
	if (original_exception_hook) {
		original_exception_hook(ex);
	}

	if (!atomic_load_explicit(&frankenphp_debugger_enabled,
	                          memory_order_relaxed)) {
		return;
	}

	int mode = atomic_load_explicit(&frankenphp_debugger_exception_mode,
	                                memory_order_relaxed);
	if (mode == 0) {
		return;
	}

	bool caught = frankenphp_debugger_is_exception_caught();
	bool should_break = false;
	if ((mode & 1) && !caught) {
		should_break = true;
	}
	if ((mode & 2) && caught) {
		should_break = true;
	}
	if (!should_break) {
		return;
	}

	const char *class_name = ZSTR_VAL(ex->ce->name);

	zval rv;
	zval *msg_zv = zend_read_property_ex(
	    zend_get_exception_base(ex), ex,
	    ZSTR_KNOWN(ZEND_STR_MESSAGE), 1, &rv);
	const char *message = "";
	if (msg_zv && Z_TYPE_P(msg_zv) == IS_STRING) {
		message = Z_STRVAL_P(msg_zv);
	}

	zend_execute_data *ed = EG(current_execute_data);
	const char *filename = "";
	uint32_t lineno = 0;
	if (ed && ed->func && ZEND_USER_CODE(ed->func->type) &&
	    ed->opline) {
		filename = ZSTR_VAL(ed->func->op_array.filename);
		lineno = ed->opline->lineno;
	}

	frankenphp_debugger_pause_exception(filename, lineno, class_name,
	                                     message);
}

void frankenphp_debugger_pause_exception(const char *filename,
                                          uint32_t lineno,
                                          const char *exception_class,
                                          const char *exception_message) {
	int idx = (int)thread_index;
	dbg_mode = DBG_PAUSED;

#ifdef ZEND_MAX_EXECUTION_TIMERS
	zend_unset_timeout();
#endif

	if (dbg_captured_frames) {
		frankenphp_debugger_free_stack(dbg_captured_frames,
		                               dbg_captured_depth);
	}
	dbg_captured_frames = frankenphp_debugger_get_stack(&dbg_captured_depth);

	if (dbg_captured_locals) {
		frankenphp_debugger_free_locals(dbg_captured_locals,
		                                dbg_captured_locals_count);
	}
	dbg_captured_locals =
	    frankenphp_debugger_get_locals(&dbg_captured_locals_count);

	if (dbg_captured_globals) {
		frankenphp_debugger_free_globals(dbg_captured_globals,
		                                 dbg_captured_globals_count);
	}
	dbg_captured_globals =
	    frankenphp_debugger_get_globals(&dbg_captured_globals_count);

	thread_pause_cmd[idx] = -1;
	go_debugger_notify_exception((GoUintptr)thread_index,
	                              (char *)filename, (GoUint32)lineno,
	                              (char *)exception_class,
	                              (char *)exception_message);

	pthread_mutex_lock(&thread_pause_mu[idx]);
	while (thread_pause_cmd[idx] < 0) {
		pthread_cond_wait(&thread_pause_cond[idx],
		                  &thread_pause_mu[idx]);
	}
	int cmd = thread_pause_cmd[idx];
	thread_pause_cmd[idx] = -1;
	pthread_mutex_unlock(&thread_pause_mu[idx]);

	switch (cmd) {
	case 1:
		dbg_mode = DBG_STEP_OVER;
		dbg_step_depth = frankenphp_debugger_get_stack_depth();
		dbg_step_start_line = lineno;
		dbg_step_start_file = filename;
		break;
	case 2:
		dbg_mode = DBG_STEP_INTO;
		dbg_step_start_line = lineno;
		dbg_step_start_file = filename;
		break;
	case 3:
		dbg_mode = DBG_STEP_OUT;
		dbg_step_depth = frankenphp_debugger_get_stack_depth();
		dbg_step_start_line = 0;
		dbg_step_start_file = NULL;
		break;
	default:
		dbg_mode = DBG_RUNNING;
		dbg_step_start_line = 0;
		dbg_step_start_file = NULL;
		break;
	}

	if (dbg_captured_frames) {
		frankenphp_debugger_free_stack(dbg_captured_frames,
		                               dbg_captured_depth);
		dbg_captured_frames = NULL;
		dbg_captured_depth = 0;
	}
	if (dbg_captured_locals) {
		frankenphp_debugger_free_locals(dbg_captured_locals,
		                                dbg_captured_locals_count);
		dbg_captured_locals = NULL;
		dbg_captured_locals_count = 0;
	}
	if (dbg_captured_globals) {
		frankenphp_debugger_free_globals(dbg_captured_globals,
		                                 dbg_captured_globals_count);
		dbg_captured_globals = NULL;
		dbg_captured_globals_count = 0;
	}

#ifdef ZEND_MAX_EXECUTION_TIMERS
	if (PG(max_input_time) != -1) {
		zend_set_timeout(INI_INT("max_execution_time"), 0);
	}
#endif
}

void frankenphp_debugger_init(void) {
	original_execute_ex = zend_execute_ex;
	zend_execute_ex = frankenphp_debugger_execute_ex;

	zend_set_user_opcode_handler(ZEND_EXT_STMT,
	                             frankenphp_debugger_ext_stmt_handler);

	original_exception_hook = zend_throw_exception_hook;
	zend_throw_exception_hook = frankenphp_debugger_exception_hook;

	breakpoint_capacity = 32;
	breakpoints = calloc(breakpoint_capacity, sizeof(frankenphp_breakpoint_t));

	for (int i = 0; i < MAX_DEBUG_THREADS; i++) {
		pthread_mutex_init(&thread_pause_mu[i], NULL);
		pthread_cond_init(&thread_pause_cond[i], NULL);
		thread_pause_cmd[i] = -1;
	}
}

void frankenphp_debugger_init_thread(int idx) {
	if (idx >= 0 && idx < MAX_DEBUG_THREADS) {
		thread_pause_cmd[idx] = -1;
	}
}

void frankenphp_debugger_resume_thread(int thread_idx, int cmd) {
	if (thread_idx < 0 || thread_idx >= MAX_DEBUG_THREADS) {
		return;
	}
	pthread_mutex_lock(&thread_pause_mu[thread_idx]);
	thread_pause_cmd[thread_idx] = cmd;
	pthread_cond_signal(&thread_pause_cond[thread_idx]);
	pthread_mutex_unlock(&thread_pause_mu[thread_idx]);
}

static void free_breakpoint_fields(frankenphp_breakpoint_t *bp);

void frankenphp_debugger_shutdown(void) {
	if (original_execute_ex) {
		zend_execute_ex = original_execute_ex;
		original_execute_ex = NULL;
	}
	zend_set_user_opcode_handler(ZEND_EXT_STMT, NULL);

	zend_throw_exception_hook = original_exception_hook;
	original_exception_hook = NULL;

	pthread_rwlock_wrlock(&bp_lock);
	for (int i = 0; i < breakpoint_count; i++) {
		free_breakpoint_fields(&breakpoints[i]);
	}
	free(breakpoints);
	breakpoints = NULL;
	breakpoint_count = 0;
	breakpoint_capacity = 0;
	pthread_rwlock_unlock(&bp_lock);
}

int frankenphp_debugger_add_breakpoint(const char *filename, uint32_t lineno,
                                        const char *condition,
                                        const char *hit_condition,
                                        const char *log_message) {
	pthread_rwlock_wrlock(&bp_lock);

	if (breakpoint_count >= breakpoint_capacity) {
		breakpoint_capacity = breakpoint_capacity ? breakpoint_capacity * 2 : 32;
		breakpoints = realloc(breakpoints,
		                      breakpoint_capacity *
		                          sizeof(frankenphp_breakpoint_t));
	}

	int id = next_breakpoint_id++;
	frankenphp_breakpoint_t *bp = &breakpoints[breakpoint_count];
	char resolved[PATH_MAX];
	if (realpath(filename, resolved) != NULL) {
		bp->filename = strdup(resolved);
	} else {
		bp->filename = strdup(filename);
	}
	bp->lineno = lineno;
	bp->id = id;
	bp->enabled = true;
	bp->condition = (condition && condition[0]) ? strdup(condition) : NULL;
	bp->hit_condition = (hit_condition && hit_condition[0]) ? strdup(hit_condition) : NULL;
	bp->log_message = (log_message && log_message[0]) ? strdup(log_message) : NULL;
	bp->hit_count = 0;
	breakpoint_count++;

	pthread_rwlock_unlock(&bp_lock);
	return id;
}

static void free_breakpoint_fields(frankenphp_breakpoint_t *bp) {
	free(bp->filename);
	free(bp->condition);
	free(bp->hit_condition);
	free(bp->log_message);
}

bool frankenphp_debugger_remove_breakpoint(int breakpoint_id) {
	pthread_rwlock_wrlock(&bp_lock);
	for (int i = 0; i < breakpoint_count; i++) {
		if (breakpoints[i].id == breakpoint_id) {
			free_breakpoint_fields(&breakpoints[i]);
			breakpoints[i] = breakpoints[breakpoint_count - 1];
			breakpoint_count--;
			pthread_rwlock_unlock(&bp_lock);
			return true;
		}
	}
	pthread_rwlock_unlock(&bp_lock);
	return false;
}

void frankenphp_debugger_clear_breakpoints(void) {
	pthread_rwlock_wrlock(&bp_lock);
	for (int i = 0; i < breakpoint_count; i++) {
		free_breakpoint_fields(&breakpoints[i]);
	}
	breakpoint_count = 0;
	pthread_rwlock_unlock(&bp_lock);
}

int frankenphp_debugger_get_stack_depth(void) {
	int depth = 0;
	zend_execute_data *ex = EG(current_execute_data);
	while (ex) {
		if (ex->func && ZEND_USER_CODE(ex->func->type)) {
			depth++;
		}
		ex = ex->prev_execute_data;
	}
	return depth;
}

frankenphp_debug_frame_t *frankenphp_debugger_get_stack(int *out_depth) {
	int depth = 0;
	zend_execute_data *ex = EG(current_execute_data);

	zend_execute_data *tmp = ex;
	while (tmp) {
		if (tmp->func && ZEND_USER_CODE(tmp->func->type)) {
			depth++;
		}
		tmp = tmp->prev_execute_data;
	}
	*out_depth = depth;

	if (depth == 0) {
		return NULL;
	}

	frankenphp_debug_frame_t *frames =
	    calloc(depth, sizeof(frankenphp_debug_frame_t));
	int i = 0;
	while (ex && i < depth) {
		if (ex->func && ZEND_USER_CODE(ex->func->type)) {
			frames[i].filename =
			    ZSTR_VAL(ex->func->op_array.filename);
			frames[i].function_name =
			    ex->func->common.function_name
			        ? ZSTR_VAL(ex->func->common.function_name)
			        : "{main}";
			frames[i].class_name =
			    ex->func->common.scope
			        ? ZSTR_VAL(ex->func->common.scope->name)
			        : NULL;
			frames[i].lineno =
			    ex->opline ? ex->opline->lineno : 0;

			int nv = ex->func->op_array.last_var;
			frames[i].num_vars = nv;
			if (nv > 0) {
				frames[i].vars = calloc(
				    nv, sizeof(frankenphp_debug_variable_t));
				for (int j = 0; j < nv; j++) {
					frames[i].vars[j].name = ZSTR_VAL(
					    ex->func->op_array.vars[j]);
					zval *val =
					    ZEND_CALL_VAR_NUM(ex, j);
					ZVAL_DEREF(val);
					frames[i].vars[j].type =
					    Z_TYPE_P(val);
					frames[i].vars[j].value = val;
				}
			}
			i++;
		}
		ex = ex->prev_execute_data;
	}
	return frames;
}

void frankenphp_debugger_free_stack(frankenphp_debug_frame_t *frames,
                                    int depth) {
	if (!frames) {
		return;
	}
	for (int i = 0; i < depth; i++) {
		free(frames[i].vars);
	}
	free(frames);
}

frankenphp_debug_variable_t *frankenphp_debugger_get_locals(int *out_count) {
	zend_execute_data *ex = EG(current_execute_data);
	if (!ex || !ex->func || !ZEND_USER_CODE(ex->func->type)) {
		*out_count = 0;
		return NULL;
	}

	zend_op_array *op_array = &ex->func->op_array;
	int count = op_array->last_var;
	*out_count = count;

	if (count == 0) {
		return NULL;
	}

	frankenphp_debug_variable_t *vars =
	    calloc(count, sizeof(frankenphp_debug_variable_t));
	for (int i = 0; i < count; i++) {
		vars[i].name = ZSTR_VAL(op_array->vars[i]);
		zval *val = ZEND_CALL_VAR_NUM(ex, i);
		ZVAL_DEREF(val);
		vars[i].type = Z_TYPE_P(val);
		vars[i].value = val;
	}
	return vars;
}

void frankenphp_debugger_free_locals(frankenphp_debug_variable_t *vars,
                                     int count) {
	free(vars);
}

frankenphp_debug_variable_t *frankenphp_debugger_get_globals(int *out_count) {
	zend_array *ht = &EG(symbol_table);
	if (!ht) {
		*out_count = 0;
		return NULL;
	}

	int count = zend_hash_num_elements(ht);
	if (count == 0) {
		*out_count = 0;
		return NULL;
	}

	frankenphp_debug_variable_t *vars =
	    calloc(count, sizeof(frankenphp_debug_variable_t));
	int i = 0;
	zend_string *key;
	zval *val;
	ZEND_HASH_FOREACH_STR_KEY_VAL(ht, key, val)
	{
		if (Z_TYPE_P(val) == IS_INDIRECT)
			val = Z_INDIRECT_P(val);
		ZVAL_DEREF(val);

		if (key) {
			vars[i].name = strdup(ZSTR_VAL(key));
		} else {
			vars[i].name = strdup("?");
		}
		vars[i].type = Z_TYPE_P(val);
		vars[i].value = val;
		i++;
	}
	ZEND_HASH_FOREACH_END();

	*out_count = i;
	return vars;
}

void frankenphp_debugger_free_globals(frankenphp_debug_variable_t *vars,
                                      int count) {
	if (!vars)
		return;
	for (int i = 0; i < count; i++) {
		free((void *)vars[i].name);
	}
	free(vars);
}

const char *frankenphp_debugger_object_class_name(zval *obj) {
	if (!obj || Z_TYPE_P(obj) != IS_OBJECT)
		return "";
	return ZSTR_VAL(Z_OBJCE_P(obj)->name);
}

frankenphp_debug_variable_t *frankenphp_debugger_object_vars(zval *obj,
                                                             int *out_count) {
	*out_count = 0;
	if (!obj || Z_TYPE_P(obj) != IS_OBJECT)
		return NULL;

	zend_array *ht =
	    zend_get_properties_for(obj, ZEND_PROP_PURPOSE_DEBUG);
	if (!ht)
		return NULL;

	int count = zend_hash_num_elements(ht);
	if (count == 0) {
		zend_release_properties(ht);
		return NULL;
	}

	frankenphp_debug_variable_t *vars =
	    calloc(count, sizeof(frankenphp_debug_variable_t));
	int i = 0;
	zend_string *key;
	zval *val;
	ZEND_HASH_FOREACH_STR_KEY_VAL(ht, key, val)
	{
		if (Z_TYPE_P(val) == IS_INDIRECT)
			val = Z_INDIRECT_P(val);
		ZVAL_DEREF(val);

		if (key) {
			const char *name = ZSTR_VAL(key);
			size_t len = ZSTR_LEN(key);
			// demangle private/protected
			if (len > 0 && name[0] == '\0') {
				const char *real =
				    memchr(name + 1, '\0', len - 1);
				if (real)
					name = real + 1;
			}
			vars[i].name = strdup(name);
		} else {
			vars[i].name = strdup("?");
		}
		vars[i].type = Z_TYPE_P(val);
		vars[i].value = val;
		i++;
	}
	ZEND_HASH_FOREACH_END();

	zend_release_properties(ht);
	*out_count = i;
	return vars;
}

frankenphp_debug_variable_t *frankenphp_debugger_array_vars(zval *arr,
                                                            int *out_count) {
	*out_count = 0;
	if (!arr || Z_TYPE_P(arr) != IS_ARRAY)
		return NULL;

	zend_array *ht = Z_ARRVAL_P(arr);
	if (!ht)
		return NULL;

	int count = zend_hash_num_elements(ht);
	if (count == 0)
		return NULL;

	frankenphp_debug_variable_t *vars =
	    calloc(count, sizeof(frankenphp_debug_variable_t));
	int i = 0;
	zend_string *key;
	zend_ulong idx;
	zval *val;
	ZEND_HASH_FOREACH_KEY_VAL(ht, idx, key, val)
	{
		if (Z_TYPE_P(val) == IS_INDIRECT)
			val = Z_INDIRECT_P(val);
		ZVAL_DEREF(val);

		if (key) {
			vars[i].name = strdup(ZSTR_VAL(key));
		} else {
			char buf[24];
			snprintf(buf, sizeof(buf), "[%lu]",
			         (unsigned long)idx);
			vars[i].name = strdup(buf);
		}
		vars[i].type = Z_TYPE_P(val);
		vars[i].value = val;
		i++;
	}
	ZEND_HASH_FOREACH_END();

	*out_count = i;
	return vars;
}

void frankenphp_debugger_free_array_vars(frankenphp_debug_variable_t *vars,
                                         int count) {
	if (!vars)
		return;
	for (int i = 0; i < count; i++) {
		free((void *)vars[i].name);
	}
	free(vars);
}

const char *frankenphp_debugger_resource_type(zval *res) {
	if (!res || Z_TYPE_P(res) != IS_RESOURCE)
		return "unknown";
	const char *type_name =
	    zend_rsrc_list_get_rsrc_type(Z_RES_P(res));
	return type_name ? type_name : "unknown";
}

int frankenphp_debugger_resource_id(zval *res) {
	if (!res || Z_TYPE_P(res) != IS_RESOURCE)
		return -1;
	return Z_RES_P(res)->handle;
}
