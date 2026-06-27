#!/usr/bin/env bash
# Генерация TypeScript-типов из openapi.yaml в shared/api-types/ts.
# Использует openapi-typescript (через npx). Типы НЕ редактируются руками.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/shared/api-types/ts/schema.ts"
SPEC="$ROOT/openapi.yaml"

echo "Generating TS types -> $OUT"
npx -y openapi-typescript@7 "$SPEC" -o "$OUT"
echo "Done."
