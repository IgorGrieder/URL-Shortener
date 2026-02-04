#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(git rev-parse --show-toplevel)
BASE_DIR="${ROOT_DIR%/*}"

usage() {
  echo "usage: worktree.sh add <name> <branch> | rm <name> | list | cleanup"
}

case "${1:-}" in
  add)
    name=${2:-}
    branch=${3:-}
    if [[ -z "$name" || -z "$branch" ]]; then
      usage
      exit 1
    fi
    path="${BASE_DIR}/${name}"
    if git show-ref --verify --quiet "refs/heads/${branch}"; then
      git worktree add "$path" "$branch"
    elif git show-ref --verify --quiet "refs/remotes/origin/${branch}"; then
      git worktree add -b "$branch" "$path" "origin/${branch}"
    else
      git worktree add -b "$branch" "$path"
    fi
    echo "created: ${path}"
    ;;
  rm)
    name=${2:-}
    if [[ -z "$name" ]]; then
      usage
      exit 1
    fi
    git worktree remove "${BASE_DIR}/${name}"
    echo "removed: ${BASE_DIR}/${name}"
    ;;
  list)
    git worktree list
    ;;
  cleanup)
    git worktree prune
    git gc --prune=now
    ;;
  *)
    usage
    exit 1
    ;;
esac
