#!/usr/bin/env bash
# Uses only administrator-provisioned binaries. Never installs packages or disables Chrome sandbox.
set -euo pipefail
umask 077
fail() { echo "$1" >&2; exit 1; }
[[ "$(uname -s)" == Linux ]] || fail 'The server desktop requires Linux.'
[[ "$(id -u)" != 0 ]] || fail 'Run as the dedicated unprivileged desktop user.'
desktop_root="${TREND_DESKTOP_STATE_ROOT:-/var/lib/kol-trend-browser}"
chrome_bin="${TREND_BROWSER_EXECUTABLE:-/usr/bin/google-chrome-stable}"
novnc_root="${TREND_NOVNC_ROOT:-/usr/share/novnc}"
vnc_password_file="${TREND_VNC_PASSWORD_FILE:-$desktop_root/vnc.pass}"
[[ "$desktop_root" == /* && -d "$desktop_root" && ! -L "$desktop_root" && -O "$desktop_root" ]] || fail 'Provision the private desktop state directory first.'
[[ "$(stat -c '%a' "$desktop_root")" == 700 ]] || fail 'Desktop state directory must have mode 0700.'
[[ "$chrome_bin" == /* && -f "$chrome_bin" && -x "$chrome_bin" ]] || fail 'Provision approved Google Chrome and set its absolute executable path.'
[[ "$novnc_root" == /* && -f "$novnc_root/vnc.html" ]] || fail 'Provision approved noVNC assets.'
[[ -f "$vnc_password_file" && ! -L "$vnc_password_file" && -O "$vnc_password_file" ]] || fail 'Provision a VNC password file owned by the desktop user.'
[[ "$(stat -c '%a' "$vnc_password_file")" == 600 ]] || fail 'VNC password file must have mode 0600.'
for dependency in xvfb-run Xvfb xauth x11vnc websockify openbox flock; do
  command -v "$dependency" >/dev/null || fail "Missing approved system dependency: $dependency"
done
if [[ "${1:-}" == --check ]]; then
  echo 'Desktop prerequisites and private directory permissions are ready. No services started.'
  exit 0
fi
if [[ $# == 0 ]]; then
  exec xvfb-run -a -s '-screen 0 1440x1000x24 -nolisten tcp' "$0" --in-display
fi
[[ $# == 1 && "$1" == --in-display && -n "${DISPLAY:-}" && -n "${XAUTHORITY:-}" ]] || fail 'Invalid desktop launch mode.'
exec 9>"$desktop_root/session.lock"
flock -n 9 || fail 'The desktop is already running.'
profile_dir="$desktop_root/chrome-profile"
mkdir -p "$profile_dir"
[[ -d "$profile_dir" && ! -L "$profile_dir" && -O "$profile_dir" && "$(stat -c '%a' "$profile_dir")" == 700 ]] || fail 'Chrome profile directory must be private and owned by the desktop user.'
children=()
cleanup() {
  trap - EXIT INT TERM
  for child in "${children[@]}"; do kill -TERM "$child" 2>/dev/null || true; done
  for child in "${children[@]}"; do wait "$child" 2>/dev/null || true; done
}
trap cleanup EXIT
trap 'exit 143' TERM INT
# Suppress remote desktop/browser diagnostic output: it can include page URLs or user input.
openbox --sm-disable >/dev/null 2>&1 & children+=("$!")
x11vnc -display "$DISPLAY" -auth "$XAUTHORITY" -localhost -rfbport 5901 -rfbauth "$vnc_password_file" -forever -shared >/dev/null 2>&1 & children+=("$!")
websockify --web="$novnc_root" 127.0.0.1:6080 127.0.0.1:5901 >/dev/null 2>&1 & children+=("$!")
# This is a normal headed Chrome. Remote debugging is enabled manually in Chrome's UI after login.
"$chrome_bin" --user-data-dir="$profile_dir" --no-first-run --no-default-browser-check 'https://ads.tiktok.com/creative/creativeCenter/trends/hashtag?period=30&region=US' >/dev/null 2>&1 & children+=("$!")
echo 'Server Chrome desktop started; access it through the authenticated HTTPS gateway.'
set +e
wait -n "${children[@]}"
set -e
echo 'A desktop component stopped; shutting down this desktop instance for service recovery.' >&2
exit 1
