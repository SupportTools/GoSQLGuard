#!/usr/bin/env bash
# Guard for charts/gosqlguard/values.yaml.template (cluster-services-1505).
#
# The Publish job regenerates values.yaml from values.yaml.template with envsubst,
# so the template - not values.yaml - is what the published chart ships. The two
# drifted apart once (the template nested s3 under settings.storage.s3 while the
# templates read .Values.settings.s3), and the published chart silently dropped
# every S3 setting. This script fails when that can happen again:
#   1. the template must equal values.yaml except for the image tag line;
#   2. a chart rendered from the envsubst'd template must carry the template's S3
#      values into the S3_* env vars.
# Needs: bash, diff, envsubst, helm. Runs offline (the mysql subchart is vendored).
set -euo pipefail

chart="charts/gosqlguard"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

# 1. No drift apart from the tag line.
grep -v '^  tag: ' "$chart/values.yaml" > "$work/values.notag"
grep -v '^  tag: ' "$chart/values.yaml.template" > "$work/template.notag"
if ! diff -u "$work/values.notag" "$work/template.notag"; then
  echo "FAIL: $chart/values.yaml.template differs from values.yaml (beyond the image tag line)." >&2
  echo "      Apply the same change to both files." >&2
  exit 1
fi

# 2. Render the chart exactly as Publish would and check the S3 env wiring.
cp -r "$chart" "$work/chart"
IMAGE_TAG="0.0.0-check" envsubst < "$chart/values.yaml.template" > "$work/chart/values.yaml"
helm template check "$work/chart" > "$work/rendered.yaml"

# Expected values come from settings.s3 in the template itself (two-space "  s3:"
# block), not from --set, so a template that puts s3 anywhere else fails here.
tpl_s3() {
  awk -v key="$1" '
    /^  s3:$/ { in_s3 = 1; next }
    in_s3 && /^  [^ ]/ { in_s3 = 0 }
    in_s3 && $1 == key":" { v = $2; gsub(/"/, "", v); print v; exit }
  ' "$chart/values.yaml.template"
}

fail=0
for pair in S3_BUCKET=bucket S3_REGION=region S3_ENDPOINT=endpoint; do
  name="${pair%%=*}"
  want="$(tpl_s3 "${pair#*=}")"
  if [ -z "$want" ]; then
    echo "FAIL: settings.s3.${pair#*=} not found in values.yaml.template" >&2
    fail=1
    continue
  fi
  got="$(grep -A1 -E "^ *- name: ${name}\$" "$work/rendered.yaml" | sed -n 's/^ *value: "\{0,1\}\([^"]*\)"\{0,1\}$/\1/p' | head -n1)"
  if [ "$got" != "$want" ]; then
    echo "FAIL: ${name} rendered as '${got}', want '${want}'" >&2
    fail=1
  fi
done
grep -q 'image: "supporttools/gosqlguard:0.0.0-check"' "$work/rendered.yaml" \
  || { echo "FAIL: image tag from IMAGE_TAG did not reach the Deployment" >&2; fail=1; }
[ "$fail" -eq 0 ] || exit 1
echo "OK: values.yaml.template matches values.yaml and its S3 values reach the S3_* env vars"
