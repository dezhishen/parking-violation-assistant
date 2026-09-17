#!/usr/bin/env bash
# 开发模式进程管理：同时启动后端（Go）与前端（Vite dev server）。
#
#   scripts/dev.sh start    启动（会先停掉上一次启动的进程）
#   scripts/dev.sh stop     停止
#   scripts/dev.sh status   查看运行状态
#
# 约定：
#   - PID 记录在 .dev/ 下：backend.pid / frontend.pid
#   - 后端与前端日志都直接输出到当前控制台，不写日志文件
#   - 后端二进制构建到 .dev/backend，数据目录随之落在 .dev/data
set -uo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR" || exit 1

DEV_DIR="$ROOT_DIR/.dev"
BACKEND_PID_FILE="$DEV_DIR/backend.pid"
FRONTEND_PID_FILE="$DEV_DIR/frontend.pid"
BACKEND_BIN="$DEV_DIR/backend"
FRONTEND_DIR="$ROOT_DIR/frontend"
DIST_INDEX="$FRONTEND_DIR/dist/index.html"

BACKEND_PORT="${BACKEND_PORT:-8080}"
FRONTEND_PORT="${FRONTEND_PORT:-5173}"

# 本实例启动的进程 PID：退出时只清理自己的进程，避免影响后续启动的实例
SERVICE_PID=""
STARTED_BACKEND_PID=""
STARTED_FRONTEND_PID=""

C_INFO=$'\033[36m'
C_WARN=$'\033[33m'
C_ERR=$'\033[31m'
C_RESET=$'\033[0m'

info() { printf '%s[dev]%s %s\n' "$C_INFO" "$C_RESET" "$*"; }
warn() { printf '%s[dev]%s %s\n' "$C_WARN" "$C_RESET" "$*" >&2; }
err() { printf '%s[dev]%s %s\n' "$C_ERR" "$C_RESET" "$*" >&2; }

pid_alive() { [ -n "${1:-}" ] && kill -0 "$1" 2>/dev/null; }

read_pid() {
	local file="$1" pid
	[ -f "$file" ] || return 1
	pid="$(tr -cd '0-9' <"$file")"
	[ -n "$pid" ] || return 1
	printf '%s' "$pid"
}

# kill_group 结束整个进程组：先 TERM，超时后 KILL
kill_group() {
	local pid="$1" name="$2" i
	pid_alive "$pid" || return 0

	info "停止 $name（pid=$pid）"
	kill -TERM -"$pid" 2>/dev/null || kill -TERM "$pid" 2>/dev/null || true
	for i in $(seq 1 40); do
		pid_alive "$pid" || break
		sleep 0.1
	done
	if pid_alive "$pid"; then
		warn "$name 未在 4s 内退出，强制结束"
		kill -KILL -"$pid" 2>/dev/null || kill -KILL "$pid" 2>/dev/null || true
		sleep 0.2
	fi
	return 0
}

stop_one() {
	local pidfile="$1" name="$2" pid
	if pid="$(read_pid "$pidfile")"; then
		kill_group "$pid" "$name"
	fi
	rm -f "$pidfile"
}

# stop_if_owned 只清理本实例启动的进程，避免旧实例误杀新实例（重复执行 make dev）。
stop_if_owned() {
	local pidfile="$1" pid="$2" name="$3" cur
	[ -n "$pid" ] || return 0
	if cur="$(read_pid "$pidfile" 2>/dev/null)" && [ "$cur" = "$pid" ]; then
		kill_group "$pid" "$name"
		rm -f "$pidfile"
	fi
	return 0
}

# do_stop 依据 PID 文件停止进程（make dev-stop / 启动前的清理使用）
do_stop() {
	mkdir -p "$DEV_DIR"
	# 先停前端再停后端，避免前端继续把请求代理到已关闭的后端
	stop_one "$FRONTEND_PID_FILE" "前端(Vite)"
	stop_one "$BACKEND_PID_FILE" "后端(Go)"
	return 0
}

# cleanup_started 只停止本实例启动的进程
cleanup_started() {
	stop_if_owned "$FRONTEND_PID_FILE" "$STARTED_FRONTEND_PID" "前端(Vite)"
	stop_if_owned "$BACKEND_PID_FILE" "$STARTED_BACKEND_PID" "后端(Go)"
	return 0
}

# start_service <pidfile> <名称> <命令...>
# 用 setsid 让服务独占进程组，PID 由服务自身写入，便于整组停止。
# 成功后把 PID 写入全局 SERVICE_PID。
start_service() {
	local pidfile="$1" name="$2" pid="" i
	shift 2
	SERVICE_PID=""
	rm -f "$pidfile"

	if command -v setsid >/dev/null 2>&1; then
		setsid bash -c 'echo $$ > "$1"; shift; exec "$@"' _ "$pidfile" "$@" &
	else
		bash -c 'echo $$ > "$1"; shift; exec "$@"' _ "$pidfile" "$@" &
	fi

	for i in $(seq 1 50); do
		pid="$(read_pid "$pidfile" 2>/dev/null || true)"
		[ -n "$pid" ] && break
		sleep 0.1
	done

	if [ -z "$pid" ]; then
		err "$name 启动失败：未获取到 PID"
		return 1
	fi
	if ! pid_alive "$pid"; then
		err "$name 启动失败（进程已退出），请查看上方日志"
		rm -f "$pidfile"
		return 1
	fi

	info "$name 已启动（pid=$pid）"
	SERVICE_PID="$pid"
	return 0
}

ensure_frontend_deps() {
	[ -d "$FRONTEND_DIR/node_modules" ] && return 0
	info "首次运行，安装前端依赖 ..."
	if ! (cd "$FRONTEND_DIR" && npm install --no-audit --no-fund); then
		err "前端依赖安装失败"
		return 1
	fi
	return 0
}

# go:embed 要求 frontend/dist 目录必须存在，缺失时生成占位页面
ensure_frontend_dist() {
	[ -e "$DIST_INDEX" ] && return 0
	mkdir -p "$(dirname "$DIST_INDEX")"
	cat >"$DIST_INDEX" <<-EOF
		<!doctype html>
		<html lang="zh-CN">
		<head>
		  <meta charset="utf-8" />
		  <title>停车违停助手（开发模式）</title>
		  <script>location.replace('http://localhost:${FRONTEND_PORT}/')</script>
		</head>
		<body>
		  开发模式下请访问 <a href="http://localhost:${FRONTEND_PORT}/">http://localhost:${FRONTEND_PORT}/</a>
		</body>
		</html>
	EOF
	info "已生成占位 $DIST_INDEX（go:embed 需要 frontend/dist 存在）"
	return 0
}

on_signal() {
	echo
	info "收到退出信号，正在停止开发进程 ..."
	cleanup_started
	exit 0
}

do_start() {
	mkdir -p "$DEV_DIR"

	# 1. 停掉上一次启动的进程
	do_stop

	# 2. 准备依赖与 embed 目录，并编译后端
	ensure_frontend_dist || exit 1
	ensure_frontend_deps || exit 1

	info "编译后端 -> $BACKEND_BIN"
	if ! go build -o "$BACKEND_BIN" .; then
		err "后端编译失败"
		exit 1
	fi

	# 3. 启动后端（日志直接输出到当前控制台）
	info "启动后端 http://127.0.0.1:$BACKEND_PORT"
	if ! start_service "$BACKEND_PID_FILE" "后端(Go)" \
		"$BACKEND_BIN" -port "$BACKEND_PORT" -no-browser; then
		exit 1
	fi
	STARTED_BACKEND_PID="$SERVICE_PID"

	# 4. 启动前端（日志同控制台输出）
	info "启动前端 http://localhost:$FRONTEND_PORT"
	if ! start_service "$FRONTEND_PID_FILE" "前端(Vite)" \
		bash -c "cd '$FRONTEND_DIR' && exec npm run dev -- --port $FRONTEND_PORT --strictPort"; then
		err "前端启动失败，正在回滚 ..."
		cleanup_started
		exit 1
	fi
	STARTED_FRONTEND_PID="$SERVICE_PID"

	trap on_signal INT TERM HUP

	echo
	info "开发模式已就绪：前端 http://localhost:$FRONTEND_PORT ｜ 后端 http://127.0.0.1:$BACKEND_PORT"
	info "日志统一输出在本控制台；按 Ctrl-C 停止，或另开终端执行 make dev-stop"
	echo

	wait

	info "本实例的开发进程已退出"
	cleanup_started
}

do_status() {
	local name pidfile pid
	while IFS='|' read -r name pidfile; do
		if pid="$(read_pid "$pidfile")" && pid_alive "$pid"; then
			printf '%s[dev]%s %s 运行中（pid=%s）\n' "$C_INFO" "$C_RESET" "$name" "$pid"
		else
			printf '%s[dev]%s %s 未运行\n' "$C_INFO" "$C_RESET" "$name"
		fi
	done <<-EOF
		后端(Go)|$BACKEND_PID_FILE
		前端(Vite)|$FRONTEND_PID_FILE
	EOF
	return 0
}

case "${1:-}" in
start) do_start ;;
stop) do_stop ;;
status) do_status ;;
restart)
	do_stop
	do_start
	;;
*)
	echo "用法: $0 {start|stop|status|restart}" >&2
	exit 2
	;;
esac
