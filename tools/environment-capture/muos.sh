#!/bin/sh
# Grout Environment Capture Script
# Run on-device via SSH to capture filesystem layout for unit test fixtures
# Usage: ssh root@device 'sh -s' < tools/env-capture.sh > device-snapshot.txt

echo "=== Device Info ==="
echo "arch: $(uname -m)"
echo "cfw: $CFW"
echo "hostname: $(hostname)"
echo "date: $(date)"

echo ""
echo "=== muOS Version ==="
cat /opt/muos/config/version.txt 2>/dev/null || echo "not muOS"

echo ""
echo "=== Storage Paths ==="
echo "storage_symlink: $(readlink -f /run/muos/storage 2>/dev/null || echo 'N/A')"
echo "mmc_exists: $([ -d /mnt/mmc ] && echo yes || echo no)"
echo "sdcard_exists: $([ -d /mnt/sdcard ] && echo yes || echo no)"
echo "union_exists: $([ -d /mnt/union ] && echo yes || echo no)"

echo ""
echo "=== Disk Space ==="
df -h /mnt/mmc 2>/dev/null | tail -1
df -h /mnt/sdcard 2>/dev/null | tail -1
df -h /run/muos/storage 2>/dev/null | tail -1

echo ""
echo "=== Save Base Path ==="
SAVE_BASE="/run/muos/storage/save"
echo "path: $SAVE_BASE"
echo "exists: $([ -d $SAVE_BASE ] && echo yes || echo no)"

echo ""
echo "=== Save Directory Structure (top 2 levels) ==="
if [ -d "$SAVE_BASE" ]; then
    find "$SAVE_BASE" -maxdepth 2 -type d | sort
fi

echo ""
echo "=== Save Files by Emulator (count + sample) ==="
if [ -d "$SAVE_BASE/file" ]; then
    for emu_dir in "$SAVE_BASE/file/"*/; do
        [ -d "$emu_dir" ] || continue
        emu_name=$(basename "$emu_dir")
        file_count=$(find "$emu_dir" -maxdepth 1 -type f ! -name ".*" | wc -l)
        sample=$(find "$emu_dir" -maxdepth 1 -type f ! -name ".*" -print | head -3)
        backup_exists=$([ -d "$emu_dir/.backup" ] && echo yes || echo no)
        echo "--- $emu_name ($file_count files, backup_dir: $backup_exists) ---"
        echo "$sample"
    done
fi

echo ""
echo "=== ROM Directory ==="
ROM_BASE="/mnt/union/ROMS"
echo "path: $ROM_BASE"
echo "exists: $([ -d $ROM_BASE ] && echo yes || echo no)"
if [ -d "$ROM_BASE" ]; then
    echo "platforms:"
    ls -1 "$ROM_BASE" | head -30
fi

echo ""
echo "=== BIOS Directory ==="
BIOS_BASE="/run/muos/storage/bios"
echo "path: $BIOS_BASE"
echo "exists: $([ -d $BIOS_BASE ] && echo yes || echo no)"
if [ -d "$BIOS_BASE" ]; then
    echo "contents:"
    ls -1 "$BIOS_BASE" | head -20
fi

echo ""
echo "=== Grout Install ==="
for path in /mnt/mmc/MUOS/application/Grout /mnt/sdcard/MUOS/application/Grout; do
    if [ -d "$path" ]; then
        echo "install_path: $path"
        echo "files:"
        ls -la "$path"
        echo ""
        echo "config.json:"
        cat "$path/config.json" 2>/dev/null | sed 's/"password":"[^"]*"/"password":"***"/g; s/"token":"[^"]*"/"token":"***"/g'
        echo ""
        echo "grout.log (last 30 lines):"
        tail -30 "$path/grout.log" 2>/dev/null || echo "no log"
        break
    fi
done

echo ""
echo "=== Save Extensions in Use ==="
if [ -d "$SAVE_BASE/file" ]; then
    find "$SAVE_BASE/file" -maxdepth 2 -type f ! -name ".*" | sed 's/.*\.//' | sort | uniq -c | sort -rn | head -20
fi

echo ""
echo "=== Done ==="
