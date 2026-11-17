#!/usr/bin/env bash
# The simulator screenshots, for .github/workflows/ios.yml and update-ios-snapshots.yml.
#
#   screenshots.sh devices [store]   choose Xcode and the two simulators and start booting them;
#                                    prints phone=, ipad= and pinned= lines for $GITHUB_OUTPUT
#   screenshots.sh shoot PHONE IPAD APP OUT
#                                    install APP on both and shoot every page into OUT, with
#                                    the environment they were taken in (environment.json)
#
# The checked-in baselines (apps/ios/snapshots/) record the Xcode, runtime and devices they were
# taken with (environment.json). `devices` takes the same ones when the runner has them, and says
# pinned=true; otherwise (or with `store`, or before any baselines exist) it takes the image's
# default Xcode, its newest iOS runtime's iPhone 17 and 11-inch iPad Pro, and says pinned=false,
# since shots from another runtime differ in ways no page changed.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
recorded=$here/snapshots/environment.json

devices() {
  local store=${1:-}
  local xcode="" runtime="" phone_name="" ipad_name="" pinned=false
  if [ -z "$store" ] && [ -f "$recorded" ]; then
    # Each value as scripts/ios-snapshots.py allows it (no newlines: they reach ::warning:: lines).
    field() {
      jq -er --arg k "$1" '.[$k] | select(type == "string" and test("^[A-Za-z0-9 ._()-]{1,100}$"))' "$recorded" \
        || { echo "::error::$recorded: $1 is missing or malformed" >&2; exit 1; }
    }
    xcode=$(field xcode)
    runtime=$(field runtime)
    phone_name=$(field phone)
    ipad_name=$(field ipad)
    xcode=${xcode// /_}
    # "26.1_17B55": the version and build of the Xcode the baselines were taken with.
    local found=""
    for app in /Applications/Xcode*.app; do
      local plist=$app/Contents/version.plist
      [ -f "$plist" ] || continue
      local have
      have="$(plutil -extract CFBundleShortVersionString raw -o - "$plist")_$(plutil -extract ProductBuildVersion raw -o - "$plist")"
      if [ "$have" = "$xcode" ]; then found=$app; break; fi
    done
    if [ -n "$found" ]; then
      sudo xcode-select -s "$found/Contents/Developer"
      pinned=true
    else
      echo "::warning::The iOS baselines were taken with Xcode ${xcode/_/ build }, which this runner lacks; the screenshots will not be compared. Add the update-ios-snapshots label to a pull request to take them again on this image." >&2
    fi
  fi
  xcodebuild -version >&2
  xcrun simctl list devices available -j > "${RUNNER_TEMP:-/tmp}/devices.json"
  local sdk_version
  sdk_version=$(xcrun --sdk iphonesimulator --show-sdk-version)
  pick() {
    python3 - "${RUNNER_TEMP:-/tmp}/devices.json" "$1" "$2" "$3" "$4" "$sdk_version" <<'PY'
import json, sys
path, family, preferred, runtime, exact_name, sdk_version = sys.argv[1:]
devices = json.load(open(path))["devices"]
if runtime:
    if runtime not in devices:
        sys.exit(0)
    found = [d for d in devices[runtime] if d["name"] == exact_name]
else:
    # The newest runtime this Xcode's simulator SDK can run that has the family: an image can
    # carry a runtime newer than its default Xcode, whose devices xcodebuild cannot see.
    version = lambda k: [int(x) for x in k.rsplit("iOS-", 1)[1].split("-")]
    sdk = [int(x) for x in sdk_version.split(".")]
    ios = sorted((k for k in devices if "iOS" in k and version(k) <= sdk
                  and any(d["name"].startswith(family) for d in devices[k])), key=version)
    if not ios:
        sys.exit(0)
    runtime = ios[-1]
    found = [d for d in devices[runtime] if d["name"].startswith(family)]
    exact = [d for d in found if d["name"] == preferred]
    found = exact or [d for d in found if d["name"].startswith(preferred)] or found
if found:
    print(found[0]["udid"], runtime, found[0]["name"], sep="\t")
PY
  }
  local phone="" ipad=""
  if [ "$pinned" = true ]; then
    phone=$(pick iPhone '' "$runtime" "$phone_name")
    ipad=$(pick iPad '' "$runtime" "$ipad_name")
    if [ -z "$phone" ] || [ -z "$ipad" ]; then
      echo "::warning::The iOS baselines were taken on $phone_name and $ipad_name with $runtime, which this runner lacks; the screenshots will not be compared. Add the update-ios-snapshots label to a pull request to take them again on this image." >&2
      pinned=false
    fi
  fi
  if [ "$pinned" != true ]; then
    # A manual run with `store` takes the sizes App Store Connect asks for instead: the 6.9-inch
    # iPhone (1320 x 2868) and the 13-inch iPad (2064 x 2752).
    if [ -n "$store" ]; then
      phone=$(pick iPhone 'iPhone 17 Pro Max' '' '') ipad=$(pick iPad 'iPad Pro 13' '' '')
    else
      phone=$(pick iPhone 'iPhone 17' '' '') ipad=$(pick iPad 'iPad Pro 11' '' '')
    fi
  fi
  if [ -z "$phone" ] || [ -z "$ipad" ]; then
    echo "::error::No iOS $sdk_version-or-older simulator runtime here has both an iPhone and an iPad." >&2
    exit 1
  fi
  local phone_id=${phone%%$'\t'*} ipad_id=${ipad%%$'\t'*}
  xcrun simctl list devices | grep -E "$phone_id|$ipad_id" >&2
  # A simulator's first boot takes minutes, so both start now, behind the builds, and the
  # screenshots wait for them (simctl bootstatus).
  for sim in "$phone_id" "$ipad_id"; do
    nohup xcrun simctl boot "$sim" > "${RUNNER_TEMP:-/tmp}/boot-$sim.log" 2>&1 &
  done
  echo "phone=$phone_id"
  echo "ipad=$ipad_id"
  echo "pinned=$pinned"
}

shoot() {
  local phone=$1 ipad=$2 app=$3 out=$4
  local id=org.orthodoxwest.office
  mkdir -p "$out"
  # The app's clock and zone, and the reader's language, are fixed with today; nothing else in
  # it moves at rest (the crown plays only when tapped, home's light only on a turn of the hour seen).
  export SIMCTL_CHILD_TZ=America/New_York
  local frames=${RUNNER_TEMP:-/tmp}/frames
  mkdir -p "$frames"
  # Installed once on each simulator; before every shot, `fresh` puts the app back as an install
  # leaves it. An earlier shot's kept reading place, way back, settings or prayed marks would
  # otherwise open a later one differently. A reinstall did this too, but took 15 to 40 seconds a
  # shot on the runners, most of the screenshots' time.
  fresh() {
    local sim=$1 data
    xcrun simctl terminate "$sim" "$id" > /dev/null 2>&1 || true
    data=$(xcrun simctl get_app_container "$sim" "$id" data)
    # The defaults through cfprefsd, which may still hold the last launch's in memory and would
    # write them back over a file merely deleted; then everything else the app keeps (scene
    # storage, caches, the screenshot marker), as the install made it.
    xcrun simctl spawn "$sim" defaults delete "$data/Library/Preferences/$id" > /dev/null 2>&1 || true
    rm -rf "$data/Library" "$data/Documents" "$data/tmp"
    mkdir -p "$data/Library/Preferences" "$data/Library/Caches" "$data/Documents" "$data/tmp"
  }
  # Each page at a fixed day (DATE, else 15 March 2026), with today pinned to it, at 9 AM.
  one() {
    local sim=$1 name=$2
    shift 2
    local day=${DATE:-2026-03-15}
    local began=$SECONDS
    fresh "$sim"
    xcrun simctl launch "$sim" "$id" \
      -AppleLanguages '(en)' -AppleLocale en_US -today "$day" -date "$day" -clock 9 "$@" > /dev/null
    local launched=$SECONDS
    # Shot once the app says the page asked for is drawn (ScreenshotReady.swift: composed, laid
    # out, scrolled to its anchor), which `fresh` emptying the temporary directory makes this
    # launch's own; then once it has come to rest, two frames a second apart alike, so an
    # arrival's fade or a panel's drop has finished. Until it is drawn the page can be the bare
    # wall, as still as a finished one, so stillness alone is not enough.
    # A launch on the runner's simulators now and then stalls before its first frame, on any
    # page and at any place in the run: most pages say they are drawn at once, but 20 to 59
    # seconds have been seen, and one master run's home took just over a minute. So the wait
    # allows three minutes, counted by the clock from the launch, and a launch that still has
    # not drawn leaves its screen, the app's log and whether it was running beside the shots.
    local data marker deadline=180
    data=$(xcrun simctl get_app_container "$sim" "$id" data)
    marker="$data/tmp/screenshot-ready"
    until [ -f "$marker" ]; do
      if [ $((SECONDS - launched)) -ge "$deadline" ]; then
        echo "::error::$name: the app did not say its page was drawn within $deadline seconds of its launch" >&2
        xcrun simctl io "$sim" screenshot "$out/$name-not-drawn.png" > /dev/null || true
        {
          echo "# simctl spawn launchctl list (the app's line, if it was running)"
          xcrun simctl spawn "$sim" launchctl list 2>&1 | grep -F "$id" || echo "not running"
          echo "# the app's log since its install"
          xcrun simctl spawn "$sim" log show --style compact --last "$((SECONDS - began + 10))s" \
            --predicate 'process == "Office"' 2>&1 | tail -n 400
        } > "$out/$name-not-drawn.log" || true
        exit 1
      fi
      sleep 0.25
    done
    local drawn=$((SECONDS - launched))
    local last="$frames/$sim-last.png" this="$frames/$sim-this.png" settled=0
    sleep 1
    xcrun simctl io "$sim" screenshot "$last" > /dev/null
    while :; do
      sleep 1
      settled=$((settled + 1))
      xcrun simctl io "$sim" screenshot "$this" > /dev/null
      if cmp -s "$last" "$this"; then break; fi
      if [ "$settled" -ge 20 ]; then
        echo "::warning::$name did not come to rest in 20 seconds after it was drawn" >&2
        break
      fi
      mv "$this" "$last"
    done
    mv "$this" "$out/$name.png"
    # What the page measured of itself (ScreenshotGeometry), beside the shot and in the log, so a
    # shot that differs from its baseline says which measure moved (#891).
    if cp "$data/tmp/screenshot-geometry.txt" "$out/$name.geometry.txt" 2> /dev/null; then
      sed "s/^/    $name /" "$out/$name.geometry.txt"
    fi
    echo "$(date +%T) $name (cleared and launched in $((launched - began))s, drawn after ${drawn}s, at rest $((settled + 1))s later)"
  }
  for sim in "$phone" "$ipad"; do
    echo "$(date +%T) waiting for $sim to boot"
    xcrun simctl bootstatus "$sim" -b > /dev/null
    echo "$(date +%T) booted"
    xcrun simctl ui "$sim" appearance light
    # Dynamic Type at its default, whatever the image set.
    xcrun simctl ui "$sim" content_size large
    xcrun simctl status_bar "$sim" override --time 9:41 --batteryState charged --batteryLevel 100 --wifiBars 3 --cellularBars 4
    xcrun simctl uninstall "$sim" "$id" > /dev/null 2>&1 || true
    xcrun simctl install "$sim" "$app"
  done
  # The phone's shots, then the iPad's. (Taken at once, each simulator's launches waited on the
  # other's, and the pair took longer than the two one after the other.)
  phone_shots() {
    for theme in nave apse; do
      one "$phone" "home-$theme" -page home -theme "$theme"
      for hour in lauds vespers compline; do
        one "$phone" "$hour-$theme" -page hour -hour "$hour" -theme "$theme"
      done
      one "$phone" "psalm-$theme" -page hour -hour vespers -anchor psalm -theme "$theme"
      one "$phone" "hymn-$theme" -page hour -hour vespers -anchor hymn -theme "$theme"
      one "$phone" "ordo-$theme" -page ordo -theme "$theme"
      one "$phone" "year-$theme" -page year -theme "$theme"
      one "$phone" "reminders-$theme" -page reminders -theme "$theme"
      one "$phone" "about-$theme" -page about -theme "$theme"
      one "$phone" "menu-$theme" -page hour -hour compline -menu YES -theme "$theme"
    done
    # The ordo scrolled to a day, its month heading pinned under the sensor housing.
    one "$phone" "ordo-day-nave" -page ordo -day YES -theme nave
    # A saint's feast page, and Lent's Vespers hymn sung from its notation.
    one "$phone" "feast-nave" -page feast -feast st-benedict -theme nave
    one "$phone" "chant-nave" -page hour -hour vespers -anchor score -chant YES -theme nave
    # Eastertide's warmer gilding.
    DATE=2026-04-12 one "$phone" "easter-nave" -page hour -hour lauds -theme nave
    # A first-class day's hour in its frame.
    for theme in nave apse; do
      DATE=2026-12-25 one "$phone" "christmas-$theme" -page hour -hour lauds -theme "$theme"
    done
  }
  # The desktop composition on an iPad.
  ipad_shots() {
    for theme in nave apse; do
      one "$ipad" "ipad-home-$theme" -page home -theme "$theme"
      one "$ipad" "ipad-vespers-$theme" -page hour -hour vespers -theme "$theme"
      one "$ipad" "ipad-ordo-$theme" -page ordo -theme "$theme"
    done
    one "$ipad" "ipad-year-nave" -page year -theme nave
    one "$ipad" "ipad-feast-nave" -page feast -feast st-benedict -theme nave
    one "$ipad" "ipad-chant-nave" -page hour -hour vespers -anchor score -chant YES -theme nave
    one "$ipad" "ipad-settings-nave" -page home -theme nave -settings YES
  }
  phone_shots
  ipad_shots
  # What they were taken with, which the baselines keep and the next run's `devices` follows.
  python3 - "$phone" "$ipad" "$out/environment.json" <<'PY'
import json, plistlib, subprocess, sys
phone, ipad, path = sys.argv[1:]
developer = subprocess.run(["xcode-select", "-p"], capture_output=True, text=True, check=True).stdout.strip()
with open(developer.rsplit("/Contents/", 1)[0] + "/Contents/version.plist", "rb") as f:
    version = plistlib.load(f)
devices = json.loads(subprocess.run(["xcrun", "simctl", "list", "devices", "-j"], capture_output=True, text=True, check=True).stdout)["devices"]
where = {d["udid"]: (runtime, d["name"]) for runtime, ds in devices.items() for d in ds}
env = {
    "xcode": f'{version["CFBundleShortVersionString"]} {version["ProductBuildVersion"]}',
    "runtime": where[phone][0],
    "phone": where[phone][1],
    "ipad": where[ipad][1],
}
if where[ipad][0] != env["runtime"]:
    sys.exit(f"The iPhone and the iPad run different runtimes: {env['runtime']}, {where[ipad][0]}")
with open(path, "w") as f:
    json.dump(env, f, indent=2)
    f.write("\n")
print(json.dumps(env))
PY
}

case "${1:-}" in
  devices) shift; devices "$@" ;;
  shoot) shift; shoot "$@" ;;
  *) echo "usage: $0 devices [store] | shoot PHONE IPAD APP OUT" >&2; exit 2 ;;
esac
