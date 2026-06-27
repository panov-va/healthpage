#!/usr/bin/env bash
# Генерация Go-типов из openapi.yaml в shared/api-types/go.
#
# Наш контракт — OpenAPI 3.1, а oapi-codegen его пока не поддерживает. Поэтому
# на лету конвертируем спецификацию в 3.0 во ВРЕМЕННЫЙ файл (источник истины
# openapi.yaml не меняется) и генерируем типы из него. Типы НЕ редактируются
# руками (CLAUDE.md §7).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SPEC="$ROOT/openapi.yaml"
GODIR="$ROOT/shared/api-types/go"
OAPI_VERSION="v2.4.1"

TMP="$(mktemp -t openapi-30.XXXXXX.yaml)"
trap 'rm -f "$TMP"' EXIT

echo "Down-converting 3.1 -> 3.0 (temp, source spec untouched)"
npx -y @apiture/openapi-down-convert@latest --input "$SPEC" --output "$TMP"

cat > "$GODIR/oapi-codegen.yaml" <<'YAML'
package: apitypes
output: apitypes.gen.go
generate:
  models: true
YAML

echo "Generating Go types -> $GODIR/apitypes.gen.go"
cd "$GODIR"
go run "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$OAPI_VERSION" \
  -config oapi-codegen.yaml "$TMP"

go mod tidy
echo "Done."
