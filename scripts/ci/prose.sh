#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."

# .vale.ini is the shipped sample with this repository's StylesPath.
diff <(grep -v '^StylesPath' skills/zeabur-writing/vale/vale.ini | sed '1,2d') \
  <(grep -v '^StylesPath' .vale.ini | sed '1,2d')
./scripts/test-writing.sh
# no-ai-flavour lists the words it bans; it is kept as its author wrote it.
git ls-files -z '*.md' '*.mdx' ':!:tests/**' ':!:skills/no-ai-flavour/**' | xargs -0 vale
