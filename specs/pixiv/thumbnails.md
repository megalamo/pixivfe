<!-- LLMs: use sentence case, keep the formatting simple (no emojis!), and maintain a technical tone -->

# pixiv thumbnails specification

This document defines the URL structure and transformation rules for pixiv thumbnail generation.

NOTE: `i.pximg.net` requires a `Referer` header for ALL requests without exception, returning HTTP 403 otherwise. The referer check accepts any `*.pixiv.net` domain (e.g., `https://www.pixiv.net/`, `https://sketch.pixiv.net`), is case-insensitive, and tolerates variations in scheme (`http` or `https`) and trailing slashes.

## URL structure

pixiv thumbnail URLs follow two main patterns.

The primary structure for `img-master`, `img-original`, and `custom-thumb` categories is:

```
https://i.pximg.net[/{size_quality}]/{category}/img/{date_path}/{id}_{page}[_{filename_suffix}].{extension}
```

Where `{date_path}` follows the format `YYYY/MM/DD/HH/MM/SS`.

A secondary structure is used for the `imgaz` category, which hosts some older pixivision article thumbnails:

```
https://i.pximg.net/imgaz/upload/{date}/{id}.{extension}
```

Where `{date}` follows the format `YYYYMMDD` and `{id}` is an internal identifier, not the public artwork ID. This structure does not support page numbers or filename suffixes. It is unknown if size transformations can be applied.

## Primary structure components

This section describes the components of the primary URL structure.

### Size quality segment

The `{size_quality}` component is OPTIONAL and MUST follow the format `/c/{width}x{height}[_{quality}][_{modifiers}]` when present.

**CRITICAL**: Size, quality, and modifier combinations are extremely restrictive. Only specific predefined combinations are functional:

- `{width}` and `{height}` MUST be integers from a very limited set of allowed square dimensions.
- **Rectangular dimensions are NOT supported** (except for special pixivision crop parameters).
- `{quality}` parameter compatibility is severely limited.
- **Modifier ordering is strict**: For example, `/c/250x250_80_a2` is valid, but `/c/250x250_a2_80` returns HTTP 403.
- **WebP modifiers**: Only specific combinations like `1200x1200_80_webp` and `540x540_10_webp` are known to work. They force the output format to `image/webp` regardless of the file extension in the URL. However, the extension in the URL must still adhere to the requirements for its category (e.g., `.jpg` for `img-master`).
- Invalid combinations result in HTTP 403, not HTTP 404.

### Category

The `{category}` component is REQUIRED and MUST be one of:

- `img-master`
- `img-original`
- `custom-thumb`
- `imgaz`

The `custom-thumb` category is not universally available. It requires the `_custom1200` filename suffix, appears to only exist for specific artworks, and can be combined with size transformations (e.g., `/c/600x600/custom-thumb/...`).

The `imgaz` category uses a distinct URL structure for pixivision article thumbnails, as detailed in the URL structure section.

### Page identifier

The `{page}` component is REQUIRED and MUST follow the format `p{page_number}` where `{page_number}` is zero-indexed.

### Filename suffix

The `{filename_suffix}` component depends on the category used:

- `img-master` category: MUST use `_master1200` or `_square1200` suffix. While both are valid for a given artwork, they are not functionally equivalent and produce different image results. Analysis with `scripts/compare_thumbnail_variants.sh` confirms they are visually different and reveals distinct transformation rules:
  - `_master1200`: Preserves the original aspect ratio by scaling the image so its longest dimension is 1200 pixels. For a `2000x1142` original, this produces a `1200x686` image. ImageMagick's `identify` reports a high JPEG quality setting (e.g., 100).
  - `_square1200`: Creates a square thumbnail by cropping and/or scaling. The logic depends on the original aspect ratio and the final dimensions are not always 1200x1200. For a wide `2000x1142` original, it can produce a `1142x1142` image (cropped based on height). For a tall `2591x3624` original, it produces a `1200x1200` image (scaled and cropped). This variant consistently has a lower JPEG quality setting (e.g., 92) and a significantly smaller file size than the `_master1200` version.
- `custom-thumb` category: MUST use `_custom1200` suffix.
- `img-original` category: MUST NOT include any filename suffix.

Violation of these rules results in an HTTP 404 error.

### File extension

The `{file_extension}` component is REQUIRED and MUST follow these rules:

- For `img-original` category: MAY be `.png` or `.jpg`. The correct extension must be determined by trial.
- When applying a size transformation to an `img-original` URL, the extension MUST match the artwork's original format. A transformed PNG remains a PNG.
- For all other categories (`img-master`, `custom-thumb`): MUST be `.jpg`. This rule is strict and applies even when using modifiers like `_webp` that change the output MIME type.

## Size variants

**IMPORTANT**: Size and quality combinations are extremely restrictive. Only specific predefined combinations work - arbitrary dimensions or quality values result in HTTP 403.

### Working size combinations (confirmed):

| Variant      | Size quality segment   | Status                         |
| ------------ | ---------------------- | ------------------------------ |
| Tiny         | `/c/128x128`           | ✓ Working                      |
| Tiny         | `/c/150x150`           | ✓ Working                      |
| Small        | `/c/240x240`           | ✓ Working                      |
| Tiny+Q       | `/c/250x250_80_a2`     | ✓ Working                      |
| Small+Q      | `/c/360x360_70`        | ✓ Working                      |
| Medium       | `/c/600x600`           | ✓ Working                      |
| Large        | `/c/1200x1200`         | ✓ Working                      |
| WebP (Large) | `/c/1200x1200_80_webp` | ✓ Working (returns image/webp) |
| WebP (Large) | `/c/1200x1200_90_webp` | ✓ Working (returns image/webp) |
| WebP (Small) | `/c/540x540_10_webp`   | ✓ Working (returns image/webp) |

### Non-working combinations (return HTTP 403):

| Variant  | Size quality segment | Status     |
| -------- | -------------------- | ---------- |
| Tiny+Q   | `/c/250x250_80`      | ✗ HTTP 403 |
| Small    | `/c/250x250`         | ✗ HTTP 403 |
| Small    | `/c/360x360`         | ✗ HTTP 403 |
| Small+Q  | `/c/360x360_80`      | ✗ HTTP 403 |
| Order    | `/c/250x250_a2_80`   | ✗ HTTP 403 |
| Medium   | `/c/300x300`         | ✗ HTTP 403 |
| Medium   | `/c/480x480`         | ✗ HTTP 403 |
| Medium   | `/c/540x540`         | ✗ HTTP 403 |
| Medium+Q | `/c/600x600_90`      | ✗ HTTP 403 |
| Large    | `/c/768x768`         | ✗ HTTP 403 |
| Large+Q  | `/c/1200x1200_90`    | ✗ HTTP 403 |
| WebP     | `/c/1200x1200_webp`  | ✗ HTTP 403 |
| WebP     | `/c/600x600_webp`    | ✗ HTTP 403 |
| Rect     | `/c/1200x630`        | ✗ HTTP 403 |

### Special variants for pixivision articles:

**IMPORTANT**: Rectangular dimensions only work with complex pixivision crop parameters.

- Type 1: `/c/1200x630_q80_a2_g1_u1_icr0.093:0.014:0.938:0.758` ✓ Working
- Type 2: `/c/1200x630_q80_a2_g1_u1_icr0:0.253:1:0.637` ✓ Working

## Category and suffix rules

These rules apply to the `img-master`, `img-original`, and `custom-thumb` categories.

1.  The `img-master` category MUST be paired with a `_master1200` or `_square1200` suffix.
2.  The `custom-thumb` category MUST be paired with its `_custom1200` suffix.
3.  The `img-original` category MUST NOT include a filename suffix.
4.  The `img-original` category MUST NOT include a size quality segment when requesting the unmodified original file.
5.  When a size quality transformation is applied, `img-master` and `img-original` categories produce different results from their respective source images.
6.  **Date path validation**: The `{date_path}` MUST match the actual artwork upload date - incorrect dates result in HTTP 404.

## Original format detection

For `img-original` URLs (both original and transformed), the file extension (`.jpg` or `.png`) cannot be determined from metadata and MUST be detected by:

1.  Attempting the request with `.jpg` extension.
2.  If that returns HTTP 404, attempt with `.png` extension.
3.  The working extension indicates the original format, which must be used for all `img-original` based requests for that artwork.

**CONFIRMED**: This detection method works reliably for both original and transformed images.

## Error handling

- **HTTP 200**: Success - image found and served.
- **HTTP 403**: Forbidden - caused by:
  - Missing or invalid `Referer` header.
  - Invalid size combinations.
  - Invalid quality parameters.
  - Invalid modifier combinations or incorrect modifier ordering.
  - Rectangular dimensions without special crop parameters.
- **HTTP 404**: Not Found - caused by:
  - Invalid artwork ID.
  - Invalid page number.
  - Wrong file extension for `img-original` (or a transformation of it).
  - Incorrect date path.
  - Incorrect suffix for a given category (e.g., `img-master` without `_master1200` or `_square1200`).
  - Requesting a `custom-thumb` for an artwork that does not have one.

## Examples

```
Master large:           /img-master/img/2025/06/05/18/10/08/131206066_p0_master1200.jpg
Master square:          /img-master/img/2025/06/05/18/10/08/131206066_p0_square1200.jpg
Original JPG:           /img-original/img/2025/06/05/18/10/08/131206066_p0.jpg
Original PNG:           /img-original/img/2023/08/20/06/04/20/110992799_p0.png
Transformed JPG:        /c/600x600/img-original/img/2025/06/05/18/10/08/131206066_p0.jpg
Transformed PNG:        /c/600x600/img-original/img/2023/08/20/06/04/20/110992799_p0.png
Transformed Master:     /c/600x600/img-master/img/2025/06/05/18/10/08/131206066_p0_master1200.jpg
Transformed Square:     /c/600x600/img-master/img/2025/06/05/18/10/08/131206066_p0_square1200.jpg
WebP Master:            /c/1200x1200_80_webp/img-master/img/2025/06/05/18/10/08/131206066_p0_master1200.jpg
Pixivision list thumb:  /imgaz/upload/2019/04/16/507548544.jpg
```

## Known limitations

- **Size combinations are extremely restrictive**.
- **Quality parameters are highly limited**.
- **WebP support is minimal**: Only specific, exact combinations work.
- **Modifier ordering is strict** and most combinations fail with HTTP 403.
- **No general-purpose rectangular dimensions**: All tested rectangular sizes return HTTP 403 (except with special crop parameters).
- **No programmatic way to determine original file format** without trial requests.
- **`custom-thumb` is not universally available** for all artworks.
- **Static thumbnails for Ugoira (animated) artworks** are not available through this URL structure.
