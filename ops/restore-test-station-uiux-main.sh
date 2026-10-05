#!/usr/bin/env bash
# Code restoration only. All Git writes happen in a disposable bare repository.
set -euo pipefail
fail() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }
apply=false
fixture=''
fixture_commit=''
fixture_tree=''
while (($#)); do
  case "$1" in
    --apply-code) apply=true; shift ;;
    --fixture-repo|--fixture-backup-commit|--fixture-backup-tree)
      (($# >= 2)) || fail "Missing value for $1"
      case "$1" in
        --fixture-repo) fixture=$2 ;;
        --fixture-backup-commit) fixture_commit=$2 ;;
        --fixture-backup-tree) fixture_tree=$2 ;;
      esac
      shift 2 ;;
    --help)
      printf '%s\n' 'Usage: bash ops/restore-test-station-uiux-main.sh [--apply-code]' 'Default: verify and preview only. --apply-code restores remote main code; no deployment/database operations.' 'Local tests only: --fixture-repo /absolute/bare.git --fixture-backup-commit SHA --fixture-backup-tree SHA'
      exit 0 ;;
    *) fail "Unknown argument: $1" ;;
  esac
done
# Prevent caller Git routing, worktree/index, replace refs, and config overrides.
for var in ${!GIT_@}; do unset "$var"; done
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_NO_REPLACE_OBJECTS=1
backup_ref=refs/heads/backup/pre-main-integration-20261005-96c0818277
backup_commit=96c08182779b12c5a39ffc0ffdda9ba89ad38bde
backup_tree=9924ce15d50a43525b0c726b1b7f22663cd10afd
target=git@github.com:LeeSssong/sub2api-test-station.git
if [[ -n "$fixture" ]]; then
  [[ "$fixture" == /* && -d "$fixture" && -n "$fixture_commit" && -n "$fixture_tree" ]] || fail 'Fixtures require an absolute existing bare repo and both expected hashes'
  [[ $(git --git-dir="$fixture" rev-parse --is-bare-repository) == true ]] || fail 'Fixture must be a local bare repository'
  target=$fixture
  backup_commit=$fixture_commit
  backup_tree=$fixture_tree
elif [[ -n "$fixture_commit$fixture_tree" ]]; then
  fail 'Fixture hashes cannot override the production target'
else
  # Validate the caller repository has exactly the intended fetch AND push target.
  [[ $(git config --get-all remote.test-station.url) == "$target" ]] || fail 'Wrong or missing test-station repository URL'
  push_url=$(git config --get-all remote.test-station.pushurl || true)
  [[ -z "$push_url" || "$push_url" == "$target" ]] || fail 'Wrong test-station push URL'
  [[ $(git remote get-url --all test-station) == "$target" ]] || fail 'Unexpected test-station fetch routing'
  [[ $(git remote get-url --push --all test-station) == "$target" ]] || fail 'Unexpected test-station push routing'
fi
[[ "$backup_commit" =~ ^[0-9a-f]{40}$ && "$backup_tree" =~ ^[0-9a-f]{40}$ ]] || fail 'Expected hashes must be full SHA-1 values'
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
git init -q --bare "$tmp/repo.git"
git --git-dir="$tmp/repo.git" -c protocol.file.allow=always fetch -q --no-tags "$target" \
  refs/heads/main:refs/heads/main "$backup_ref:refs/heads/verified-backup"
actual_backup=$(git --git-dir="$tmp/repo.git" rev-parse refs/heads/verified-backup)
[[ "$actual_backup" == "$backup_commit" ]] || fail 'Backup commit does not match the verified source'
[[ $(git --git-dir="$tmp/repo.git" rev-parse 'refs/heads/verified-backup^{tree}') == "$backup_tree" ]] || fail 'Backup tree does not match the verified source'
parent=$(git --git-dir="$tmp/repo.git" rev-parse 'refs/heads/main^{commit}')
printf 'Target: %s\nCurrent main: %s\nBackup commit: %s\nRestoration tree: %s\n' "$target" "$parent" "$backup_commit" "$backup_tree"
if [[ "$apply" == false ]]; then
  printf 'DRY RUN: verified; no remote writes. Use --apply-code to create and push a restoration commit.\n'
  exit 0
fi
# pre-push sees the server advertisement. Checking it closes the window between
# fetch and push (including rewinds); receive-pack then CAS-checks the same SHA.
mkdir "$tmp/hooks"
cat > "$tmp/hooks/pre-push" <<HOOK
#!/usr/bin/env bash
set -euo pipefail
count=0
while read -r local_ref local_sha remote_ref remote_sha; do
  [[ "\$remote_ref" == refs/heads/main && "\$remote_sha" == "$parent" ]] || {
    printf 'ERROR: remote main changed; fetch and review again.\n' >&2
    exit 1
  }
  count=\$((count + 1))
done
[[ \$count == 1 ]]
HOOK
chmod +x "$tmp/hooks/pre-push"
export GIT_AUTHOR_NAME='Sub2API code restoration' GIT_AUTHOR_EMAIL='code-restoration@sub2api.invalid'
export GIT_COMMITTER_NAME="$GIT_AUTHOR_NAME" GIT_COMMITTER_EMAIL="$GIT_AUTHOR_EMAIL"
commit=$(printf 'Restore test-station code to verified pre-integration backup\n\nBackup: %s\nTree: %s\nPrevious main: %s\nCode only; database and deployment require separate recovery.\n' "$backup_commit" "$backup_tree" "$parent" | git --git-dir="$tmp/repo.git" commit-tree "$backup_tree" -p "$parent")
git --git-dir="$tmp/repo.git" -c core.hooksPath="$tmp/hooks" -c protocol.file.allow=always push "$target" "$commit:refs/heads/main"
# Verify immediately; another later writer can still advance main legitimately.
remote=$(git --git-dir="$tmp/repo.git" -c protocol.file.allow=always ls-remote "$target" refs/heads/main)
[[ "$remote" == "$commit"$'\trefs/heads/main' ]] || fail "Push completed but main changed during verification; restoration commit: $commit"
printf 'Code restoration pushed and verified: %s\nTree: %s\nDeployment and database restoration were not performed.\n' "$commit" "$backup_tree"
