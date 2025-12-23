Useful tools.

## Opengrep

Opengrep is a non-sucky version of Semgrep that [doesn't hide features](https://github.com/semgrep/semgrep/issues/10849).

Download it at [https://github.com/opengrep/opengrep/releases/](https://github.com/opengrep/opengrep/releases/).

!!! note "For Arch Linux users"
    If you get `libcrypt.so.1: cannot open shared object file`, install `libxcrypt-compat` with `sudo pacman -S libxcrypt-compat`.

Common usage:

```sh
# scan
opengrep scan -f opengrep.yaml
# scan with autofix
opengrep scan -f opengrep.yaml --autofix FILES...
```

## goimports-reviser

This tool organizes imports into three groups -- std, 3rdparty, ours.

Install with

```sh
go install -v github.com/incu6us/goimports-reviser/v3@latest
```

Format all code:

```sh
goimports-reviser -rm-unused -format ./...
```
