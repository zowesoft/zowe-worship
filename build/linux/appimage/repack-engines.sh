#!/usr/bin/env bash
# wails3's appimage generator assembles the AppDir itself and offers no hook
# for extra files, so the staged speech engine is folded in after the fact:
# copy it beside the binary in the AppDir the generator leaves behind, then
# repack with appimagetool. Skips silently when no engine was staged, matching
# the "absent -> Get engine" behaviour of the other packages.
set -euo pipefail

name="${1:?AppImage base name, e.g. zowe-worship}"
outdir="${2:?output dir for the repacked image, e.g. ../../../bin}"
engines="${3:-../../../build/engines}"

case "$(uname -m)" in
  x86_64) apparch=x86_64 ;;
  aarch64) apparch=aarch64 ;;
  *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

if [ ! -d "$engines" ]; then
  echo "no staged engines at $engines - shipping without the speech engine"
  exit 0
fi

appdir="build/$name-$apparch.AppDir"
if [ ! -d "$appdir" ]; then
  echo "AppDir $appdir not found - run 'wails3 generate appimage' first" >&2
  exit 1
fi

cp -r "$engines" "$appdir/usr/bin/engines"

tool="appimagetool-$apparch.AppImage"
if [ ! -f "$tool" ]; then
  wget -q -N "https://github.com/AppImage/appimagetool/releases/download/continuous/$tool"
  chmod +x "$tool"
fi

rm -f "$outdir/$name-$apparch.AppImage"
"$tool" --appimage-extract-and-run "$appdir" "$outdir/$name-$apparch.AppImage"
echo "engine bundled into $outdir/$name-$apparch.AppImage"
