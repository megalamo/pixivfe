#!/bin/bash

# Script to dump raw analysis output for pixiv thumbnail variants
#
# This script downloads variants of a pixiv artwork and outputs raw results from:
# - ImageMagick identify command
# - ImageMagick compare command
#
# Usage: ./compare_thumbnail_variants.sh [options]
# Options:
#   --use-examples    Use current URLs from spec examples (default)
#   --custom <artwork_id> <page> <date_path>  Use custom artwork
# Examples:
#   ./compare_thumbnail_variants.sh
#   ./compare_thumbnail_variants.sh --custom 127035726 0 2025/02/08/23/08/44

set -euo pipefail

# Check dependencies
command -v curl >/dev/null 2>&1 || {
	echo "curl is required but not installed." >&2
	exit 1
}
command -v identify >/dev/null 2>&1 || {
	echo "ImageMagick identify is required but not installed." >&2
	exit 1
}
command -v compare >/dev/null 2>&1 || {
	echo "ImageMagick compare is required but not installed." >&2
	exit 1
}

# Parse arguments
USE_EXAMPLES=true
if [ $# -eq 0 ] || [ "$1" = "--use-examples" ]; then
	USE_EXAMPLES=true
	# Use current examples from thumbnails.md spec
	ARTWORK_ID_JPG="131206066"
	PAGE_JPG="0"
	DATE_PATH_JPG="2025/06/05/18/10/08"

	ARTWORK_ID_PNG="110992799"
	PAGE_PNG="0"
	DATE_PATH_PNG="2023/08/20/06/04/20"
elif [ $# -eq 4 ] && [ "$1" = "--custom" ]; then
	USE_EXAMPLES=false
	ARTWORK_ID="$2"
	PAGE="$3"
	DATE_PATH="$4"
else
	echo "Usage: $0 [options]"
	echo "Options:"
	echo "  --use-examples    Use current URLs from spec examples (default)"
	echo "  --custom <artwork_id> <page> <date_path>  Use custom artwork"
	echo ""
	echo "Examples:"
	echo "  $0"
	echo "  $0 --use-examples"
	echo "  $0 --custom 127035726 0 2025/02/08/23/08/44"
	exit 1
fi

# Ensure date paths don't have leading/trailing slashes
if [ "$USE_EXAMPLES" = true ]; then
	DATE_PATH_JPG=$(echo "$DATE_PATH_JPG" | sed 's|^/||; s|/$||')
	DATE_PATH_PNG=$(echo "$DATE_PATH_PNG" | sed 's|^/||; s|/$||')
else
	DATE_PATH=$(echo "$DATE_PATH" | sed 's|^/||; s|/$||')
fi

# Base URL components
BASE_URL="https://i.pximg.net"

# Check if URL returns HTTP 200
check_url_status() {
	local url="$1"
	local status_code=$(curl -s -o /dev/null -w "%{http_code}" -H "Referer: ${REFERER}" "$url")
	[ "$status_code" = "200" ]
}

# Simple function to download and output raw analysis
download_and_analyze() {
	local url="$1"
	local filename="$2"
	local description="$3"

	echo "=== $description ==="
	echo "URL: $url"

	# Check if URL returns HTTP 200 before downloading
	if ! check_url_status "$url"; then
		echo "ERROR: URL does not return HTTP 200 for $description"
		echo ""
		return 1
	fi

	echo "Status: HTTP 200 ✓"
	echo ""

	if ! curl -s -H "Referer: ${REFERER}" -o "$filename" "$url"; then
		echo "ERROR: Failed to download $description"
		return 1
	fi

	# Check if download was successful
	if [ ! -s "$filename" ]; then
		echo "ERROR: $description file is empty or download failed"
		return 1
	fi

	# Raw identify output
	echo "--- identify output ---"
	identify "$filename" || echo "identify failed"
	echo ""

	echo "--- identify -verbose output ---"
	identify -verbose "$filename" || echo "identify -verbose failed"
	echo ""

	return 0
}

# Helper function to compare two images and output the result
compare_images() {
	local file1="$1"
	local file2="$2"
	local diff_file="$3"
	local description="$4"

	echo "=== $description ==="
	if compare -metric AE "$file1" "$file2" "$diff_file" 2>/dev/null; then
		# Exit code 0 means images are identical
		echo "0 (0) - Images are identical"
	else
		# Exit code 1 means images differ - capture the output
		local diff_output=$(compare -metric AE "$file1" "$file2" "$diff_file" 2>&1)
		echo "${diff_output} - Images differ significantly"
	fi
	echo ""
}

# Temporary files directory
TEMP_DIR=$(mktemp -d)
cleanup() {
	rm -rf "$TEMP_DIR"
}
trap cleanup EXIT

# Required referer header
REFERER="https://www.pixiv.net/"

if [ "$USE_EXAMPLES" = true ]; then
	echo "=== RAW PIXIV THUMBNAIL ANALYSIS ==="
	echo "Using current spec examples:"
	echo "  JPG artwork: ${ARTWORK_ID_JPG} (${DATE_PATH_JPG})"
	echo "  PNG artwork: ${ARTWORK_ID_PNG} (${DATE_PATH_PNG})"
	echo ""

	# Test JPG artwork variants
	echo "=== JPG ARTWORK (${ARTWORK_ID_JPG}) ==="

	# Define all URLs to test for JPG artwork
	JPG_MASTER="${BASE_URL}/img-master/img/${DATE_PATH_JPG}/${ARTWORK_ID_JPG}_p${PAGE_JPG}_master1200.jpg"
	JPG_SQUARE="${BASE_URL}/img-master/img/${DATE_PATH_JPG}/${ARTWORK_ID_JPG}_p${PAGE_JPG}_square1200.jpg"
	JPG_ORIGINAL="${BASE_URL}/img-original/img/${DATE_PATH_JPG}/${ARTWORK_ID_JPG}_p${PAGE_JPG}.jpg"

	# Download JPG variants and output raw analysis
	download_and_analyze "$JPG_MASTER" "${TEMP_DIR}/jpg_master.jpg" "JPG master1200"
	download_and_analyze "$JPG_SQUARE" "${TEMP_DIR}/jpg_square.jpg" "JPG square1200"
	download_and_analyze "$JPG_ORIGINAL" "${TEMP_DIR}/jpg_original.jpg" "JPG original"

	# Compare master vs square if both exist
	if [ -f "${TEMP_DIR}/jpg_master.jpg" ] && [ -f "${TEMP_DIR}/jpg_square.jpg" ]; then
		compare_images "${TEMP_DIR}/jpg_master.jpg" "${TEMP_DIR}/jpg_square.jpg" "${TEMP_DIR}/jpg_diff.png" "compare master vs square"
	fi

	echo "=== PNG ARTWORK (${ARTWORK_ID_PNG}) ==="

	# Define all URLs to test for PNG artwork
	PNG_MASTER="${BASE_URL}/img-master/img/${DATE_PATH_PNG}/${ARTWORK_ID_PNG}_p${PAGE_PNG}_master1200.jpg"
	PNG_SQUARE="${BASE_URL}/img-master/img/${DATE_PATH_PNG}/${ARTWORK_ID_PNG}_p${PAGE_PNG}_square1200.jpg"
	PNG_ORIGINAL="${BASE_URL}/img-original/img/${DATE_PATH_PNG}/${ARTWORK_ID_PNG}_p${PAGE_PNG}.png"

	# Download PNG variants and output raw analysis
	download_and_analyze "$PNG_MASTER" "${TEMP_DIR}/png_master.jpg" "PNG master1200"
	download_and_analyze "$PNG_SQUARE" "${TEMP_DIR}/png_square.jpg" "PNG square1200"
	download_and_analyze "$PNG_ORIGINAL" "${TEMP_DIR}/png_original.png" "PNG original"

	# Compare master vs square if both exist
	if [ -f "${TEMP_DIR}/png_master.jpg" ] && [ -f "${TEMP_DIR}/png_square.jpg" ]; then
		compare_images "${TEMP_DIR}/png_master.jpg" "${TEMP_DIR}/png_square.jpg" "${TEMP_DIR}/png_diff.png" "compare master vs square"
	fi

else
	# Custom artwork analysis
	echo "=== RAW ANALYSIS FOR ARTWORK ${ARTWORK_ID} ==="
	echo "Page: ${PAGE}"
	echo "Date path: ${DATE_PATH}"
	echo ""

	# Construct URLs
	MASTER_URL="${BASE_URL}/img-master/img/${DATE_PATH}/${ARTWORK_ID}_p${PAGE}_master1200.jpg"
	SQUARE_URL="${BASE_URL}/img-master/img/${DATE_PATH}/${ARTWORK_ID}_p${PAGE}_square1200.jpg"

	# Download and analyze
	download_and_analyze "$MASTER_URL" "${TEMP_DIR}/master.jpg" "master1200 variant"
	download_and_analyze "$SQUARE_URL" "${TEMP_DIR}/square.jpg" "square1200 variant"

	# Compare if both exist
	if [ -f "${TEMP_DIR}/master.jpg" ] && [ -f "${TEMP_DIR}/square.jpg" ]; then
		compare_images "${TEMP_DIR}/master.jpg" "${TEMP_DIR}/square.jpg" "${TEMP_DIR}/diff.png" "compare master vs square"
	fi
fi

echo "Files saved in: ${TEMP_DIR}"
