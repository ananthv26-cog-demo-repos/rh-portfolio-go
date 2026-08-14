#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
road_root="${PAVED_ROAD_ROOT:-$repo_root/../rh-paved-road}"
service_name="${SERVICE_NAME:-rh-portfolio-go}"
render_port="${RENDER_PORT:-8081}"
template_root="$road_root/templates/go-service"

if [[ ! -f "$template_root/Dockerfile.tmpl" || ! -f "$template_root/deploy.yaml.tmpl" ]]; then
  echo "paved-road go-service templates not found under $template_root" >&2
  exit 1
fi

mkdir -p "$repo_root/deploy"
sed -e "s/{{SERVICE_NAME}}/$service_name/g" -e "s/{{PORT}}/$render_port/g" \
  "$template_root/Dockerfile.tmpl" > "$repo_root/Dockerfile"
sed -e "s/{{SERVICE_NAME}}/$service_name/g" -e "s/{{PORT}}/$render_port/g" \
  "$template_root/deploy.yaml.tmpl" > "$repo_root/deploy/deploy.yaml"
