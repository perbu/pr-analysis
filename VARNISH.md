# Varnish Project Style Guide

This guide captures the essential coding conventions and best practices for contributing to the Varnish project. It is based on common patterns and strong preferences observed in code reviews. The overarching philosophy is to prioritize **clarity, correctness, and performance**, in that order.

## 1. Code Style (C & VCL)

Consistency is key. Adhering to the established "house style" makes the codebase easier to read and maintain for everyone.

### General Philosophy
- **Readability First:** Prioritize clear, simple, and direct code over "clever" or overly complex solutions. If a choice is between a minor optimization and significant clarity, choose clarity.
- **Simplicity:** Strive for the simplest logic that correctly solves the problem. Avoid excessive defensive code for "impossible" scenarios.
- **Atomic Changes:** Keep pull requests focused on a single logical change. A bug fix and a feature should be in separate PRs. Squash related fixup commits before merging.

### C Formatting
- **Braces:** `else` and `else if` must be on the same line as the preceding closing brace. Always use braces for multi-line blocks.
  ```c
  if (condition) {
      ...
  } else {
      ...
  }
  ```
- **Spacing:** Use a space after keywords like `if`, `for`, `while`, `switch`.
- **Pointers:** The asterisk `*` is placed next to the variable name, not the type.
  ```c
  // Correct
  char *ptr;

  // Incorrect
  char* ptr;
  ```
- **Line Breaks:** `return`, `break`, and `continue` should be on their own lines. Break long lines to maintain readability.
- **Headers:** System headers should be included before project-specific headers. `config.h` should be included explicitly where needed.

### Naming Conventions
- **Prefixes:** Global symbols and functions should be prefixed with their module name (e.g., `VBE_`, `VSA_`, `VRT_`) to avoid namespace collisions.
- **Conciseness:** Use concise, established names, especially for flags (e.g., `c`, `b`) that align with existing tools like `varnishlog`.
- **Clarity:** Variable names should be descriptive and unambiguous. Avoid overly short or cryptic names.

### VCL Style
- **Control Flow:** Avoid `return(miss)` in `vcl_hit{}` unless there is a strong, well-understood reason. Prefer explicit checks for backend health.
- **Explicitness:** New features should integrate seamlessly. Avoid requiring special-case logic in user VCL whenever possible.
- **Readability:** Avoid excessive nesting. Decompose complex logic into separate conditional blocks.

## 2. Architecture & Design

Architectural decisions should prioritize long-term maintainability, stability, and performance.

- **Separation of Concerns:** Maintain a clear separation between libraries and applications (e.g., `libvarnish` must not depend on `varnishd/cache` code). Platform-specific code should be isolated.
- **API & ABI Stability:**
    - Avoid changing existing `enum` or `const` values in public APIs.
    - When extending a public `struct`, always append new fields to the end to preserve ABI compatibility.
    - ABI breaks require a `VRT_MAJOR_VERSION` bump and strong justification.
- **VCL Integration:**
    - Expose new functionality and configuration to VCL via `std` functions, new variables, or dedicated VMODs.
- **Resource Management:**
    - Use Varnish's workspace (`WS`) for temporary, task-scoped memory. `wrk->aws` is the preferred scratchpad.
    - For tightly coupled data, consider allocating a struct and its associated data in a single `malloc` or `WS_Alloc` call.
- **Configuration:**
    - Expose configurable parameters via `varnish-cli` or VCL, not hardcoded values.
    - Prefer intelligent defaults and auto-tuning over exposing complex low-level tunables.

## 3. Error Handling & Robustness

Varnish must be resilient. Error handling should be explicit, consistent, and robust.

- **Assertions vs. Runtime Errors:**
    - Use `assert()` (and variants like `AN()`, `AZ()`, `CHECK_OBJ_NOTNULL`) for *programming errors* and to enforce internal invariants. These are conditions that should *never* happen in correct code.
    - Use explicit return codes (e.g., `errno`) and VCL failures (`VRT_fail()`) for *expected runtime errors* (e.g., invalid user input, network failure). Do not panic on VCL-triggered conditions.
- **Protocol Adherence:** Strictly follow RFC specifications. When implementing logic based on an RFC, add a comment referencing the specific section (e.g., `// rfcXXXX,l,line_start,line_end`).
- **Resource Cleanup:** Ensure all acquired resources (memory, locks, file descriptors) are released on *all* code paths, especially error paths.
- **Input Validation:** Robustly validate all external input at the earliest possible stage. For UDS backends, for example, warn on missing sockets at load time but fail if a path exists and is not a socket.

## 4. Performance Guidelines

Varnish is a high-performance cache. Performance is a primary design constraint.

- **Minimize Lock Contention:**
    - Hold locks for the shortest possible duration. Move non-critical work outside of the locked section.
    - For read-heavy patterns, check a condition without a lock first, then re-verify it after acquiring the lock.
    - Prefer `pthread_cond_signal` over `pthread_cond_broadcast` unless multiple threads must be woken.
- **Minimize System Calls:** In hot paths, reduce syscalls. For example, use `writev` to combine multiple writes into one, and implement read-ahead buffering to reduce `read` calls.
- **Optimize for the Common Case:** Design logic to be fastest for the most frequent path. For example, avoid allocations or complex checks in the success path if they are only needed for error handling.
- **Data-Driven Decisions:** Justify performance optimizations with concrete data from tools like `varnishlog` or benchmarks.

## 5. Testing Requirements

Untested code is broken code. Comprehensive testing is mandatory.

- **VTCs are Mandatory:** All new features and bug fixes must be accompanied by Varnish Test Cases (VTCs).
- **Robust VTCs:**
    - **No `sleep`:** Do not use fixed `sleep` delays. Use active synchronization mechanisms like `logexpect`, `server wait`, or dummy CLI `ping` commands.
    - **Specificity:** Tests should verify the actual outcome (e.g., client response, log content), not just server-side state.
    - **Coverage:** Test both happy paths and error conditions, including invalid inputs.
- **Sanitizers:** Code should pass cleanly with sanitizers (`asan`, `msan`). If a test fails only with a sanitizer due to timing or overhead, it can be skipped with `feature !sanitizer`, but this should be a last resort.

## 6. Documentation Standards

If it's not documented, it doesn't exist. Documentation is a core part of any feature.

- **Documentation is Critical:** Pull requests are not complete without corresponding documentation updates. This includes man pages, reference manuals, and `.vcc` files.
- **Clarity and Precision:** Use clear, correct, and unambiguous English. Proofread for typos and grammatical errors.
- **Location:** Keep documentation close to its source. VMOD function documentation belongs in the `.vcc` file. Project overview belongs in `README.rst`.
- **Release Notes (`changes.rst`):**
    - Entries should be terse, one-line summaries of user-facing changes.
    - Detailed explanations belong in the "What's New/Upgrading" documentation, linked from `changes.rst`.
- **Code Snippets:** All VCL examples in documentation must be backed by a VTC to ensure they are correct and remain so.