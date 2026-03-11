ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BINARY="$ROOT/apps/miso/bin/miso-test"
FIXTURE="$ROOT/apps/miso/test/env"

# ── colours ────────────────────────────────────────────────────────────────────
GREEN='\033[0;32m'
RED='\033[0;31m'
CYAN='\033[0;36m'
BOLD='\033[1m'
RESET='\033[0m'

pass() { printf "    ${GREEN}✓${RESET} %s\n" "$1"; }
fail() { printf "    ${RED}✗${RESET} %s\n" "$1"; }
info() { printf "    ${CYAN}→${RESET} %s\n" "$1"; }
log()  { printf "${CYAN}→${RESET} %s\n" "$1"; }

# ── build ───────────────────────────────────────────────────────────────────────
log "building miso..."
cd "$ROOT/apps/miso"
go build -o "$BINARY" ./cmd
log "built: $BINARY"
echo ""

# ── helpers ─────────────────────────────────────────────────────────────────────
PASS=0
FAIL=0

# run_pass <description> [env_override_file]
# expects miso env to exit 0
run_pass() {
  desc="$1"
  config="$2"
  if [ -n "$config" ]; then
    out=$( cd "$FIXTURE" && MISO_CONFIG="$config" "$BINARY" env 2>&1 ) && rc=0 || rc=$?
  else
    out=$( cd "$FIXTURE" && "$BINARY" env 2>&1 ) && rc=0 || rc=$?
  fi
  if [ "$rc" -eq 0 ]; then
    pass "$desc"
    PASS=$((PASS + 1))
  else
    fail "$desc"
    printf "     %s\n" "$out"
    FAIL=$((FAIL + 1))
  fi
}

# run_fail <description> <expected_substring> [env_override_file]
# expects miso env to exit non-zero and output to contain expected_substring
run_fail() {
  desc="$1"
  expected="$2"
  config="$3"
  if [ -n "$config" ]; then
    out=$( cd "$FIXTURE" && MISO_CONFIG="$config" "$BINARY" env 2>&1 ) && rc=0 || rc=$?
  else
    out=$( cd "$FIXTURE" && "$BINARY" env 2>&1 ) && rc=0 || rc=$?
  fi
  if [ "$rc" -ne 0 ] && echo "$out" | grep -qF "$expected"; then
    pass "$desc"
    PASS=$((PASS + 1))
  elif [ "$rc" -eq 0 ]; then
    fail "$desc  (expected failure, got success)"
    FAIL=$((FAIL + 1))
  else
    fail "$desc  (expected substring: '$expected')"
    printf "     got: %s\n" "$out"
    FAIL=$((FAIL + 1))
  fi
}

# ── write a temporary miso.json into the fixture dir and clean up after ─────────
write_config() { printf '%s' "$1" > "$FIXTURE/miso.json"; }
restore_config() {
  # restore the canonical fixture config from source
  cp "$ROOT/test/env/miso.json.bak" "$FIXTURE/miso.json" 2>/dev/null || true
}

# back up original config once
cp "$FIXTURE/miso.json" "$FIXTURE/miso.json.bak"
trap 'cp "$FIXTURE/miso.json.bak" "$FIXTURE/miso.json"; rm -f "$FIXTURE/miso.json.bak"' EXIT

# ── tests ───────────────────────────────────────────────────────────────────────
printf "${BOLD}env validation tests${RESET}\n\n"

# ── 1. full fixture passes ───────────────────────────────────────────────────────
info "baseline"
restore_config
run_pass "all variable types validate against fixture .env"

# ── 2. per-type spot checks (bad values) ────────────────────────────────────────
echo ""
info "type validation — bad values"

write_config '{
  "package-manager":"bun","name":"env-test","scripts":"./scripts",
  "env":[{"label":"t","path":".env.bad","variables":{"PORT":"port"}}]
}'
printf 'PORT=99999\n' > "$FIXTURE/.env.bad"
run_fail "port: rejects out-of-range value (99999)" "port must be 1-65535"

write_config '{
  "package-manager":"bun","name":"env-test","scripts":"./scripts",
  "env":[{"label":"t","path":".env.bad","variables":{"PORT":"port"}}]
}'
printf 'PORT=banana\n' > "$FIXTURE/.env.bad"
run_fail "port: rejects non-numeric value" "invalid port"

write_config '{
  "package-manager":"bun","name":"env-test","scripts":"./scripts",
  "env":[{"label":"t","path":".env.bad","variables":{"N":"int"}}]
}'
printf 'N=3.14\n' > "$FIXTURE/.env.bad"
run_fail "int: rejects float string" "invalid integer"

write_config '{
  "package-manager":"bun","name":"env-test","scripts":"./scripts",
  "env":[{"label":"t","path":".env.bad","variables":{"N":"int+"}}]
}'
printf 'N=-5\n' > "$FIXTURE/.env.bad"
run_fail "int+: rejects negative value" "must be positive integer"

write_config '{
  "package-manager":"bun","name":"env-test","scripts":"./scripts",
  "env":[{"label":"t","path":".env.bad","variables":{"U":"url"}}]
}'
printf 'U=not-a-url\n' > "$FIXTURE/.env.bad"
run_fail "url: rejects non-url string" "invalid url"

write_config '{
  "package-manager":"bun","name":"env-test","scripts":"./scripts",
  "env":[{"label":"t","path":".env.bad","variables":{"U":{"type":"url","schemes":["redis","rediss"]}}}]
}'
printf 'U=https://example.com\n' > "$FIXTURE/.env.bad"
run_fail "url: rejects disallowed scheme" "url scheme must be one of"

write_config '{
  "package-manager":"bun","name":"env-test","scripts":"./scripts",
  "env":[{"label":"t","path":".env.bad","variables":{"E":{"type":"enum","values":["a","b","c"]}}}]
}'
printf 'E=d\n' > "$FIXTURE/.env.bad"
run_fail "enum: rejects value not in list" "must be one of"

write_config '{
  "package-manager":"bun","name":"env-test","scripts":"./scripts",
  "env":[{"label":"t","path":".env.bad","variables":{"V":{"type":"pattern","pattern":"^v?\\d+\\.\\d+\\.\\d+$"}}}]
}'
printf 'V=not-semver\n' > "$FIXTURE/.env.bad"
run_fail "pattern: rejects non-matching value" "does not match pattern"

write_config '{
  "package-manager":"bun","name":"env-test","scripts":"./scripts",
  "env":[{"label":"t","path":".env.bad","variables":{"B":"bool"}}]
}'
printf 'B=maybe\n' > "$FIXTURE/.env.bad"
run_fail "bool: rejects unrecognised value" "invalid bool"

write_config '{
  "package-manager":"bun","name":"env-test","scripts":"./scripts",
  "env":[{"label":"t","path":".env.bad","variables":{"EM":"email"}}]
}'
printf 'EM=not-an-email\n' > "$FIXTURE/.env.bad"
run_fail "email: rejects invalid address" "EM"

write_config '{
  "package-manager":"bun","name":"env-test","scripts":"./scripts",
  "env":[{"label":"t","path":".env.bad","variables":{"J":"json"}}]
}'
printf 'J={bad json\n' > "$FIXTURE/.env.bad"
run_fail "json: rejects malformed JSON" "J"

write_config '{
  "package-manager":"bun","name":"env-test","scripts":"./scripts",
  "env":[{"label":"t","path":".env.bad","variables":{"ID":"uuid"}}]
}'
printf 'ID=not-a-uuid\n' > "$FIXTURE/.env.bad"
run_fail "uuid: rejects non-uuid string" "ID"

write_config '{
  "package-manager":"bun","name":"env-test","scripts":"./scripts",
  "env":[{"label":"t","path":".env.bad","variables":{"S":{"type":"string","min":5,"max":10}}}]
}'
printf 'S=hi\n' > "$FIXTURE/.env.bad"
run_fail "string: rejects value below min length" "S"

write_config '{
  "package-manager":"bun","name":"env-test","scripts":"./scripts",
  "env":[{"label":"t","path":".env.bad","variables":{"F":{"type":"float","min":0,"max":1}}}]
}'
printf 'F=1.5\n' > "$FIXTURE/.env.bad"
run_fail "float: rejects value above max" "must be <="

rm -f "$FIXTURE/.env.bad"

# ── 3. required / optional ───────────────────────────────────────────────────────
echo ""
info "required / optional"

write_config '{
  "package-manager":"bun","name":"env-test","scripts":"./scripts",
  "env":[{"label":"t","path":".env.req","variables":{"MISSING":"string"}}]
}'
printf '' > "$FIXTURE/.env.req"
run_fail "required: missing variable is an error" "missing required variable: MISSING"

write_config '{
  "package-manager":"bun","name":"env-test","scripts":"./scripts",
  "env":[{"label":"t","path":".env.req","variables":{"GONE":{"type":"string","optional":true}}}]
}'
printf '' > "$FIXTURE/.env.req"
run_pass "optional: missing optional variable passes"

write_config '{
  "package-manager":"bun","name":"env-test","scripts":"./scripts",
  "env":[{"label":"t","path":".env.req","required":"none","variables":{"ALSO_GONE":"string"}}]
}'
printf '' > "$FIXTURE/.env.req"
run_pass "required=none: absent variable passes"

write_config '{
  "package-manager":"bun","name":"env-test","scripts":"./scripts",
  "env":[{"label":"t","path":".env.req","required":["NEED_THIS"],"variables":{"NEED_THIS":"string","OPTIONAL_EXTRA":"string"}}]
}'
printf 'NEED_THIS=here\n' > "$FIXTURE/.env.req"
run_pass "required=[keys]: listed key present passes"

write_config '{
  "package-manager":"bun","name":"env-test","scripts":"./scripts",
  "env":[{"label":"t","path":".env.req","required":["NEED_THIS"],"variables":{"NEED_THIS":"string","OPTIONAL_EXTRA":"string"}}]
}'
printf 'OPTIONAL_EXTRA=here\n' > "$FIXTURE/.env.req"
run_fail "required=[keys]: listed key missing is an error" "missing required variable: NEED_THIS"

rm -f "$FIXTURE/.env.req"

# ── 4. multi-entry ───────────────────────────────────────────────────────────────
echo ""
info "multi-entry"

printf 'PORT=8080\n' > "$FIXTURE/.env.a"
printf 'REDIS_URL=redis://localhost:6379\n' > "$FIXTURE/.env.b"

write_config '{
  "package-manager":"bun","name":"env-test","scripts":"./scripts",
  "env":[
    {"label":"app","path":".env.a","variables":{"PORT":"port"}},
    {"label":"worker","path":".env.b","variables":{"REDIS_URL":{"type":"url","schemes":["redis","rediss"]}}}
  ]
}'
run_pass "multi-entry: both entries pass"

write_config '{
  "package-manager":"bun","name":"env-test","scripts":"./scripts",
  "env":[
    {"label":"app","path":".env.a","variables":{"PORT":"port"}},
    {"label":"worker","path":".env.b","variables":{"MISSING":"string"}}
  ]
}'
run_fail "multi-entry: error includes label of failing entry" "worker: missing required variable: MISSING"

rm -f "$FIXTURE/.env.a" "$FIXTURE/.env.b"

# ── 5. missing env file ──────────────────────────────────────────────────────────
echo ""
info "missing file"

write_config '{
  "package-manager":"bun","name":"env-test","scripts":"./scripts",
  "env":[{"label":"ghost","path":".env.does-not-exist","variables":{"X":"string"}}]
}'
run_fail "missing file: reports file not found" "env file not found"

# ── summary ──────────────────────────────────────────────────────────────────────
echo ""
TOTAL=$((PASS + FAIL))
if [ "$FAIL" -eq 0 ]; then
  printf "${GREEN}${BOLD}all $TOTAL tests passed${RESET}\n"
else
  printf "${RED}${BOLD}$FAIL of $TOTAL tests failed${RESET}\n"
  exit 1
fi
