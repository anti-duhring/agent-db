---
status: partial
phase: 02-runner-postgres-baseline
source: [02-VERIFICATION.md]
started: 2026-03-31T23:50:00.000Z
updated: 2026-03-31T23:50:00.000Z
---

## Current Test

[awaiting human testing]

## Tests

### 1. End-to-end benchmark run
expected: 5-row results table with P50/P95/P99 columns, all COUNT=10, all latencies non-zero
command: `go run . --backend postgres --scenario all --profile small --iterations 10 --warmup 2 --seed 42`
result: [pending]

### 2. Dry-run mode
expected: 5 `[PASS]` lines and exit 0
command: `go run . --dry-run`
result: [pending]

### 3. Scenario filtering
expected: exactly 2 rows (AppendMessage, LoadSlidingWindow), COUNT=5 each
command: `go run . --backend postgres --scenario append,window --profile small --iterations 5`
result: [pending]

### 4. Warmup exclusion
expected: COUNT=10 (not 30), confirming warmup iterations excluded from histogram
command: `go run . --backend postgres --scenario append --profile small --iterations 10 --warmup 20`
result: [pending]

## Summary

total: 4
passed: 0
issues: 0
pending: 4
skipped: 0
blocked: 0

## Gaps
