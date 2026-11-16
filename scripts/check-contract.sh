#!/usr/bin/env bash
# Цепочка контракта: TypeSpec (typespec/main.tsp) -> OpenAPI 3 (tsp-output)
# -> маршруты Go-сервера (backend/main.go).
# Сверка двусторонняя, по путям (методы внутри хендлеров — separate concern):
#  1) каждый путь OpenAPI должен попадать в какой-то mux-шаблон Go;
#  2) каждый mux-шаблон Go должен покрываться хотя бы одним путём OpenAPI.
# {param}-сегмента в Go ({id}, {sub}) матчат любой непустой сегмент.
set -euo pipefail
cd "$(dirname "$0")/.."

SPEC="typespec/tsp-output/@typespec/openapi3/openapi.json"
if [[ ! -f "$SPEC" ]]; then
  echo "нет $SPEC — выполните: mise run codegen" >&2
  exit 1
fi

node -e '
const fs = require("fs");
const spec = require("./typespec/tsp-output/@typespec/openapi3/openapi.json");
const go = fs.readFileSync("backend/main.go", "utf8");

const specPaths = Object.keys(spec.paths);
const goPaths = [
  ...new Set(
    [...go.matchAll(/"(?:GET |POST |PUT |DELETE )?(\/api\/[^"]*)"/g)].map((m) => m[1]),
  ),
];

const toRegex = (tpl) =>
  new RegExp("^" + tpl.split("/").map((seg) => (seg.startsWith("{") ? "[^/]+" : seg.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"))).join("/") + "$");

const goRegexes = goPaths.map((p) => ({ p, re: toRegex(p) }));

let bad = 0;
for (const sp of specPaths) {
  if (!goRegexes.some((g) => g.re.test(sp))) {
    console.error("путь OpenAPI не обслуживается Go: " + sp);
    bad++;
  }
}
for (const g of goRegexes) {
  if (!specPaths.some((sp) => g.re.test(sp))) {
    console.error("mux-шаблон Go не описан в OpenAPI: " + g.p);
    bad++;
  }
}
if (bad) process.exit(1);
console.log(`Контракт OK: ${specPaths.length} путей OpenAPI покрыты ${goPaths.length} mux-шаблонами Go`);
'
