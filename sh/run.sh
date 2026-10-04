# Start the first-party development entrypoint. The native server does not
# watch or rebuild files; restart this command after changing Go/templates.
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
cd "$REPO_ROOT"
exec go run ./cmd/leanote -runMode dev
