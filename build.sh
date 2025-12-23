#!/bin/sh
set -o errexit
# not posix compliant and breaks docker builds
# set -o pipefail

# Variables
# ANSI color codes
GREEN='\033[0;32m'
RESET='\033[0m'

# Function to print colored prefix
print_prefix() {
	printf "${GREEN}[build.sh]${RESET} %s\n" "$*"
}

PREFIX="[build.sh]"
BINARY_NAME="pixivfe"
GOOS=${GOOS:-$(go env GOOS)}
GOARCH=${GOARCH:-$(go env GOARCH)}

check_css() {
	SOURCE_CSS_FILE="./assets/css/tailwind-style_source.css"
	CSS_FILE="./assets/css/tailwind-style.css"

	if [ ! -f "${CSS_FILE}" ]; then
		print_prefix "${CSS_FILE} does not exist. Compiling the Tailwind CSS source now."
		if [ ! -f "${SOURCE_CSS_FILE}" ]; then
			print_prefix "${SOURCE_CSS_FILE} does not exist. Please update your source code."
			exit 1
		fi
		tailwindcss -i "$SOURCE_CSS_FILE" -o "$CSS_FILE"
	fi
}

build() {
	print_prefix "Building ${BINARY_NAME}..."
	go mod tidy

	# formattting
	go tool templ fmt -v .
	go tool wsl --default all --enable assign-exclusive,assign-expr --fix ./...
	go tool goimports-reviser -rm-unused ./...
	go tool gofumpt -l -w -extra .

	if type "jq" >/dev/null >/dev/null; then
		print_prefix "jq found."
	fi
	check_css
	if command -v python3 >/dev/null 2>&1; then
		python3 scripts/extract_icons.py --update-fonts
	else
		python scripts/extract_icons.py --update-fonts
	fi

	go tool templ generate
	go run ./cmd/genconfig
	build_binary
}

build_binary() {
	print_prefix "Building ${BINARY_NAME}..."
	CGO_ENABLED=0 go build -v -buildvcs=true -o "${BINARY_NAME}"
}

i18n_extract() {
	print_prefix "Extracting i18n strings to POT..."
	go tool templ generate
	go run ./cmd/i18n_extract
}

i18n_merge() {
	print_prefix "Merging POT into locale PO files..."
	if ! command -v msgmerge >/dev/null 2>&1; then
		print_prefix "msgmerge not found. Please install gettext tools."
		exit 1
	fi
	# For each locale PO file, merge in place.
	for f in po/*.po; do
		[ -f "$f" ] || continue
		# Do not merge the template into itself.
		if [ "${f##*/}" = "pixivfe.pot" ]; then
			continue
		fi
		msgmerge --update --backup=none "$f" po/pixivfe.pot
	done
}

i18n_validate() {
	print_prefix "Validating PO files..."
	if ! command -v msgfmt >/dev/null 2>&1; then
		print_prefix "msgfmt not found. Please install gettext tools."
		exit 1
	fi
	fail=0
	# Validate all PO files, skipping the POT template.
	for f in po/*.po; do
		[ -f "$f" ] || continue
		if [ "${f##*/}" = "pixivfe.pot" ]; then
			continue
		fi
		if ! msgfmt -cv -o /dev/null "$f"; then
			fail=1
		fi
	done
	if [ $fail -ne 0 ]; then
		print_prefix "PO validation failed."
		exit 1
	fi
	print_prefix "All PO files are valid."
}

run() {
	build
	print_prefix "Running ${BINARY_NAME}..."
	./"${BINARY_NAME}"
}

watch() {
	# cleanup() cleans up background processes.
	cleanup() {
		print_prefix "Stopping watch processes..."
		if [ -n "$TEMPL_PID" ]; then
			kill $TEMPL_PID 2>/dev/null || true
		fi
		if [ -n "$WATCHEXEC_PID" ]; then
			kill $WATCHEXEC_PID 2>/dev/null || true
		fi
		print_prefix "All watchers stopped. Exiting..."
		exit 0
	}

	# cleanup on exit
	trap cleanup EXIT

	print_prefix "Starting templ watch..."
	go tool templ generate -watch &
	TEMPL_PID=$!

	# NOTE: not reliable
	# print_prefix "Starting tailwind watch..."
	# tailwindcss -i ./assets/css/tailwind-style_source.css -o ./assets/css/tailwind-style.css --watch --minify &
	# TAILWIND_PID=$!

	print_prefix "Watching for changes to Go files and restarting application..."
	print_prefix "Press Ctrl+C to stop all watchers"
	# NOTE: `cd $(pwd)` is required to correctly pass the `build_binary` argument
	if command -v watchexec >/dev/null; then
		watchexec -c -r --exts go,templ,js -- sh -c "cd $(pwd) && ./build.sh build_binary && ./${BINARY_NAME}" &
		WATCHEXEC_PID=$!
	fi

	# Wait for all background processes
	wait
}

help() {
	print_prefix "Available commands:"
	print_prefix "  build              - Run full build process"
	print_prefix "  build_binary       - Build the binary"
	print_prefix "  check_css          - Check if CSS file exists and compile if needed"
	print_prefix "  i18n_extract       - Generate po/pixivfe.pot from source"
	print_prefix "  i18n_merge         - Merge POT into all locale PO files"
	print_prefix "  i18n_validate      - Validate all PO files with msgfmt"
	print_prefix "  run                - Build and run the binary"
	print_prefix "  watch              - Build and run the binary, restart when file changes (requires watchexec)"
	print_prefix "  help               - Show this help message"
	echo ""
}

# Function to handle command execution
execute_command() {
	case "$1" in
	build) build ;;
	build_binary) build_binary ;;
	i18n_extract) i18n_extract ;;
	i18n_merge) i18n_merge ;;
	i18n_validate) i18n_validate ;;
	check_css) check_css ;;
	run) run ;;
	watch) watch ;;
	help) help ;;
	*)
		print_prefix "Unknown command: $1"
		print_prefix "Use 'help' to see available commands"
		exit 1
		;;
	esac
}

# Main execution
if [ $# -eq 0 ]; then
	build
else
	while [ $# -ne 0 ]; do
		execute_command "$1"
		shift
	done
fi
