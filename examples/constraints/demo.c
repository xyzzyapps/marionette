/**
 * demo.c — Architectural constraint violation demo for Marionette.
 *
 * This file demonstrates constraint rules in action:
 *   - @tag required keys (security, gdpr-impact)
 *   - Code rules (forbidden: malloc, printf, unsafe, goto)
 *   - Architecture rules (module boundaries)
 *
 * Run with:
 *   marionette --db demo.db index
 *   marionette lint --strict
 *
 * Expected violations (when .marionetterc.json has matching rules):
 *   1. Missing "security" key on process_user_data()
 *   2. Missing "gdpr-impact" key on process_user_data()
 *   3. Forbidden malloc() call in buffer_helper()
 *   4. Forbidden printf() call in log_message()
 *   5. Forbidden goto usage in retry_logic()
 */

// @tag-block data-processor
// @tag domain: "user-data"
// @tag team-owner: "platform-core"
// @tag depends-on: ["db", "cache"]
void process_user_data(int user_id) {
    // @tag-inline pii: "Processing user profile — GDPR Article 6 compliance needed"
    load_profile(user_id);
    // Missing @tag security and @tag gdpr-impact — will trigger lint rules
}

// @tag-block utility
// @tag security: "internal"
// @tag gdpr-impact: "none"
// @tag review-status: "approved"
void log_message(const char *msg) {
    // @tag-inline forbidden: "printf is not allowed in production code"
    printf("LOG: %s\n", msg);  /* ❌ FORBIDDEN by code rule: no printf */
}

// @tag-block utility
// @tag security: "internal"
// @tag gdpr-impact: "none"
int* buffer_helper(size_t size) {
    // @tag-inline forbidden: "malloc usage — use arena allocator instead"
    int *buf = (int*)malloc(size * sizeof(int));  /* ❌ FORBIDDEN by code rule: no malloc */
    if (!buf) {
        /* @tag-inline forbidden: "goto usage — use structured error handling" */
        goto error_exit;  /* ❌ FORBIDDEN by code rule: no goto */
    }
    return buf;

error_exit:
    return NULL;
}

// @tag-block api-endpoint
// @tag route: "/v1/process"
// @tag security: "rbac"
// @tag gdpr-impact: "high"
// @tag layer: "controller"
// @tag module: "billing"
// @tag must-not-import: ["reporting", "notification"]
int retry_logic(int attempts) {
    for (int i = 0; i < attempts; i++) {
        if (try_operation() == 0) {
            return 0;
        }
    }
    // @tag-inline fallback: "All retries exhausted — using goto for error path"
    goto cleanup;  /* ❌ FORBIDDEN by code rule: no goto */
cleanup:
    return -1;
}

// @tag-block architecture
// @tag layer: "entrypoint"
// @tag domain: "user-management"
// @tag team-owner: "platform-core"
// @tag gdpr-impact: "critical"
int main(int argc, char **argv) {
    /* ✅ This function has all required tags */
    process_user_data(42);
    return 0;
}
