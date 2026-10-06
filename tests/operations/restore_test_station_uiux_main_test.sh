#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "$0")/../.." && pwd)
tool="$root/ops/restore-test-station-uiux-main.sh"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
export GIT_AUTHOR_NAME=Test GIT_AUTHOR_EMAIL=test@example.invalid GIT_COMMITTER_NAME=Test GIT_COMMITTER_EMAIL=test@example.invalid
git init -q --bare "$tmp/remote.git"
git init -q "$tmp/source"
printf 'backup\n' > "$tmp/source/data"
git -C "$tmp/source" add data
git -C "$tmp/source" commit -qm backup
backup=$(git -C "$tmp/source" rev-parse HEAD)
tree=$(git -C "$tmp/source" rev-parse 'HEAD^{tree}')
git -C "$tmp/source" branch backup/pre-main-integration-20261005-96c0818277
printf 'current\n' > "$tmp/source/data"
git -C "$tmp/source" commit -qam current
current=$(git -C "$tmp/source" rev-parse HEAD)
git -C "$tmp/source" push -q "$tmp/remote.git" HEAD:main refs/heads/backup/pre-main-integration-20261005-96c0818277
run() { bash "$tool" --fixture-repo "$tmp/remote.git" --fixture-backup-commit "$backup" --fixture-backup-tree "$tree" "$@"; }
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
reject() { if "$@" > "$tmp/rejected.log" 2>&1; then fail 'unsafe action succeeded'; fi; }
[[ -f "$tool" ]] || fail 'rollback tool is missing'
# Default mode rejects wrong fetch identity and mismatched/multiple push URLs before network.
(cd "$tmp/source" && reject bash "$tool")
git -C "$tmp/source" remote add test-station git@github.com:LeeSssong/sub2api-test-station.git
git -C "$tmp/source" config remote.test-station.pushurl git@github.com:LeeSssong/sub2api.git
(cd "$tmp/source" && reject bash "$tool" --apply-code)
git -C "$tmp/source" config --add remote.test-station.pushurl git@github.com:LeeSssong/sub2api-test-station.git
(cd "$tmp/source" && reject bash "$tool" --apply-code)
reject bash "$tool" --fixture-backup-commit "$backup" --fixture-backup-tree "$tree"
run > "$tmp/dry.log"
[[ $(git --git-dir="$tmp/remote.git" rev-parse main) == "$current" ]] || fail 'dry run changed main'
reject bash "$tool" --fixture-repo "$tmp/remote.git" --fixture-backup-commit "$current" --fixture-backup-tree "$tree" --apply-code
reject bash "$tool" --fixture-repo "$tmp/remote.git" --fixture-backup-commit "$backup" --fixture-backup-tree "$current" --apply-code
reject bash "$tool" --fixture-repo https://github.com/LeeSssong/sub2api-test-station.git --fixture-backup-commit "$backup" --fixture-backup-tree "$tree"
# A dirty checkout must be untouched, including index and refs.
printf 'dirty\n' > "$tmp/source/data"
printf 'untracked\n' > "$tmp/source/local-only"
before=$(git -C "$tmp/source" status --porcelain)
(cd "$tmp/source" && run --apply-code) > "$tmp/apply.log"
restored=$(git --git-dir="$tmp/remote.git" rev-parse main)
[[ $(git --git-dir="$tmp/remote.git" rev-parse 'main^{tree}') == "$tree" ]] || fail 'restored tree differs'
[[ $(git --git-dir="$tmp/remote.git" rev-parse 'main^') == "$current" ]] || fail 'history parent differs'
[[ $(git -C "$tmp/source" status --porcelain) == "$before" ]] || fail 'checkout changed'
[[ $(git -C "$tmp/source" rev-parse HEAD) == "$current" ]] || fail 'checkout ref changed'
# Remote changes between fetch and push advertisement, even a rewind, must fail.
mkdir "$tmp/bin"
real_git=$(command -v git)
cat > "$tmp/bin/git" <<WRAPPER
#!/usr/bin/env bash
for arg in "\$@"; do
  if [[ \$arg == push ]]; then
    "$real_git" --git-dir="$tmp/remote.git" update-ref refs/heads/main "$backup"
    break
  fi
done
exec "$real_git" "\$@"
WRAPPER
chmod +x "$tmp/bin/git"
reject env PATH="$tmp/bin:$PATH" bash "$tool" --fixture-repo "$tmp/remote.git" --fixture-backup-commit "$backup" --fixture-backup-tree "$tree" --apply-code
[[ $(git --git-dir="$tmp/remote.git" rev-parse main) == "$backup" ]] || fail 'rewind race overwritten'
# Remote changes after push advertisement must fail the server CAS.
git --git-dir="$tmp/remote.git" update-ref refs/heads/main "$current"
cat > "$tmp/remote.git/hooks/pre-receive" <<HOOK
#!/usr/bin/env bash
unset GIT_QUARANTINE_PATH
"$real_git" --git-dir="$tmp/remote.git" update-ref refs/heads/main "$restored"
HOOK
chmod +x "$tmp/remote.git/hooks/pre-receive"
reject run --apply-code
[[ $(git --git-dir="$tmp/remote.git" rev-parse main) == "$restored" ]] || fail 'receive race overwritten'
printf 'PASS: dry run, backup SHA/tree, local-only fixtures, repository identity, exact tree/parent, dirty checkout, advertisement rewind race, receive race\n'
