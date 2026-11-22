#!/bin/sh
# Runs when the user double-clicks the .deb in the ChromeOS Files app
# ("Install with Linux"). It hands TelTun.apk to the Android side of the
# Chromebook over ChromeOS's own adb bridge, so nobody has to type commands.
set -u
DIR=/opt/teltun-chromeos
ADB="$DIR/adb"
[ -x "$ADB" ] || ADB=$(command -v adb || true)
LOG=/var/log/teltun-chromeos-install.log
exec >>"$LOG" 2>&1
echo "--- $(date) installing TelTun"

if [ -z "$ADB" ]; then
  echo "adb not found"; exit 1
fi
export HOME=/root
"$ADB" start-server

# "arc" is the Android side of the Chromebook. The first time, ChromeOS asks
# "Allow USB debugging?" on screen: wait for the user to allow it.
ok=0
for i in $(seq 1 90); do
  "$ADB" connect arc >/dev/null 2>&1
  if [ "$("$ADB" -s arc:5555 get-state 2>/dev/null)" = "device" ]; then ok=1; break; fi
  sleep 2
done
if [ "$ok" != 1 ]; then
  echo "Android side not reachable: turn on 'Develop Android apps > ADB debugging' in Settings"
  "$ADB" kill-server
  exit 1
fi

"$ADB" -s arc:5555 install -r "$DIR/TelTun.apk"
rc=$?
"$ADB" -s arc:5555 shell monkey -p app.teltun -c android.intent.category.LAUNCHER 1 >/dev/null 2>&1 || true
"$ADB" kill-server
echo "done rc=$rc"
exit $rc
