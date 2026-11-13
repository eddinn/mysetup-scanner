#!/bin/sh
#
# MySetup scan: prints a report of this computer's hardware, for you to paste on your MySetup
# build page. The format is in docs/report-format.md.
#
#     sh mysetup-scan.sh
#
# It only reads. It uses no network, writes no files, installs nothing and does not ask for
# root unless you pass --with-root. Please read it before you run it; it is meant to be read.
#
# Several sources hold more than this report wants: udev and dmidecode also hold memory serial
# numbers, for example. So sources are read by exact key, and only the fields named in emit()
# can be printed. Serial numbers, hostnames, usernames and addresses are never asked for.
#
# Plain POSIX sh for Linux, macOS, the BSDs and other Unix-like hosts. It needs only awk, tr,
# uname and id; everything else is optional and tried in order. For Windows, use
# mysetup-scan.ps1.

set -u

VERSION=0.2.0
MAX_ITEMS=64
WITH_ROOT=no
NOTES=''
US=$(printf '\037')

# The tests point this at a made-up computer. Nothing else sets it.
ROOT=${MYSETUP_SCAN_TEST_ROOT:-}
if [ -n "$ROOT" ]; then
    PATH=$ROOT/bin
else
    PATH=${PATH:-/usr/bin:/bin}:/usr/sbin:/sbin:/usr/local/sbin:/usr/pkg/sbin
fi
LC_ALL=C
export PATH LC_ALL

# ---------------------------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------------------------

have() {
    command -v "$1" >/dev/null 2>&1
}

note() {
    NOTES="$NOTES$1
"
}

# The first line of a file, without the NUL bytes device-tree files end in.
first_line() {
    [ -r "$1" ] || return 0
    tr -d '\000' <"$1" 2>/dev/null | awk 'NR == 1 { print; exit }'
}

# field SEPARATOR KEY: the value of the first line on standard input whose key is exactly KEY.
field() {
    awk -v sep="$1" -v key="$2" '
        {
            at = index($0, sep)
            if (at == 0) next
            name = substr($0, 1, at - 1)
            gsub(/^[ \t]+|[ \t]+$/, "", name)
            if (name != key) next
            value = substr($0, at + length(sep))
            gsub(/^[ \t]+|[ \t]+$/, "", value)
            print value
            exit
        }'
}

# Printable ASCII, single spaces, at most 120 characters. Prints nothing for a placeholder such as
# "To Be Filled By O.E.M.", or for anything shaped like an address, an ID or a home directory:
# a hardware name never looks like those.
clean_text() {
    printf '%s' "$1" | tr -cd ' -~' | awk '
        BEGIN {
            count = split("to be filled by o.e.m.|to be filled by oem|default string|not applicable|" \
                "not specified|not available|n/a|na|none|null|unknown|undefined|empty|[empty]|o.e.m.|oem|" \
                "system manufacturer|system product name|system version|baseboard product name|" \
                "base board product name|no dimm|nodimm|not installed|no module installed|dimm|module|" \
                "part number|serial number|1234567890|123456789012|" \
                "system sku|sku|system family|family|type1productconfigid|type1family|default|invalid|x.x", words, "|")
            for (i = 1; i <= count; i++) placeholder[words[i]] = 1
            hh = "[0-9a-f][0-9a-f]"
            mac = hh "[:-]" hh "[:-]" hh "[:-]" hh "[:-]" hh "[:-]" hh
            uuid = hh hh hh hh "-" hh hh "-" hh hh "-" hh hh "-" hh hh hh hh hh hh
        }
        {
            gsub(/[ \t]+/, " ")
            sub(/^ /, "")
            sub(/ $/, "")
            low = tolower($0)
            if (low in placeholder) next
            if (low ~ /^0+$/ || low ~ /^0x0+$/ || low ~ /^x+$/ || low ~ /^[-. ]*$/) next
            if (low ~ /^(array[0-9]*_)?partnumber[0-9]*$/ || low ~ /^partnum[0-9]*$/) next
            if (low ~ /^manufacturer[0-9]*$/) next
            if (low ~ mac || low ~ uuid) next
            if (low ~ /[0-9]+[.][0-9]+[.][0-9]+[.][0-9]+/) next
            if (low ~ /[^ ]@[^ ]/ || index(low, "/home/") || index(low, "/users/") || index(low, "c:\\")) next
            print substr($0, 1, 120)
        }'
}

# The only way anything reaches the report. A field not named here stops the script.
emit() {
    case $1 in
        system.vendor | system.model | system.family | system.version | system.sku | \
            cpu.vendor | cpu.model | motherboard.vendor | motherboard.model | gpu.*.vendor | gpu.*.model | \
            memory.*.maker | memory.*.part | storage.*.model)
            _value=$(clean_text "$2")
            ;;
        cpu.cores | cpu.threads | memory.total_mb | memory.*.size_mb | memory.*.speed_mts | \
            memory.*.configured_speed_mts | storage.*.size_bytes)
            _value=$2
            case $_value in '' | 0 | *[!0-9]*) return 0 ;; esac
            ;;
        gpu.*.pci | gpu.*.subsystem)
            _value=$2
            case $_value in
                [0-9a-f][0-9a-f][0-9a-f][0-9a-f]:[0-9a-f][0-9a-f][0-9a-f][0-9a-f]) ;;
                *) return 0 ;;
            esac
            ;;
        source | tested | os.family | os.name | os.version | os.architecture | environment | system.chassis | \
            memory.*.type | memory.*.form | storage.*.interface | storage.*.media | capability.*)
            _value=$2
            case $_value in '' | *[!A-Za-z0-9._+/-]*) return 0 ;; esac
            # What the memory tables say when they do not know.
            case $1:$_value in memory.*.type:Other | memory.*.type:Unknown | memory.*.form:Other | memory.*.form:Unknown) return 0 ;; esac
            ;;
        *)
            printf 'mysetup-scan: internal error: no field named %s\n' "$1" >&2
            exit 70
            ;;
    esac
    [ -n "$_value" ] || return 0
    printf '%s=%s\n' "$1" "$_value"
}

# capability NAME STATE: STATE is "available", or the reason it is not.
capability() {
    if [ "$2" = available ]; then
        emit "capability.$1" available
    else
        emit "capability.$1" unavailable
        emit "capability.$1.reason" "$2"
    fi
}

# The command that installs a package here, for the member to run if they want to.
install_command() {
    _as=sudo
    have sudo || { have doas && _as=doas; }
    if have apt-get; then
        printf '%s apt install %s' "$_as" "$1"
    elif have dnf; then
        printf '%s dnf install %s' "$_as" "$1"
    elif have yum; then
        printf '%s yum install %s' "$_as" "$1"
    elif have pacman; then
        printf '%s pacman -S %s' "$_as" "$1"
    elif have zypper; then
        printf '%s zypper install %s' "$_as" "$1"
    elif have apk; then
        printf '%s apk add %s' "$_as" "$1"
    elif have xbps-install; then
        printf '%s xbps-install %s' "$_as" "$1"
    elif have emerge; then
        printf '%s emerge %s' "$_as" "$1"
    elif have opkg; then
        printf '%s opkg install %s' "$_as" "$1"
    elif have nix-shell; then
        printf 'nix-shell -p %s' "$1"
    elif have pkg; then
        printf '%s pkg install %s' "$_as" "$1"
    elif have pkg_add; then
        printf '%s pkg_add %s' "$_as" "$1"
    elif have pkgin; then
        printf '%s pkgin install %s' "$_as" "$1"
    else
        printf 'your package manager, package %s' "$1"
    fi
}

normal_architecture() {
    case $1 in
        x86_64 | amd64) echo x86-64 ;;
        aarch64 | arm64) echo arm64 ;;
        i?86) echo x86 ;;
        i86pc) echo x86-64 ;;
        arm*) echo arm ;;
        riscv64 | ppc64le | ppc64 | s390x) echo "$1" ;;
        *) echo other ;;
    esac
}

normal_cpu_vendor() {
    case $1 in
        GenuineIntel) echo Intel ;;
        AuthenticAMD) echo AMD ;;
        *) echo "$1" ;;
    esac
}

# Disks read as MODEL<US>BYTES<US>INTERFACE<US>MEDIA, one per line.
emit_storage_rows() {
    _n=0
    while IFS=$US read -r _model _bytes _interface _media; do
        [ -n "$_model$_bytes" ] || continue
        if [ "$_n" -ge "$MAX_ITEMS" ]; then
            note "Only the first $MAX_ITEMS disks are listed."
            break
        fi
        emit "storage.$_n.model" "$_model"
        emit "storage.$_n.size_bytes" "$_bytes"
        emit "storage.$_n.interface" "$_interface"
        emit "storage.$_n.media" "$_media"
        _n=$((_n + 1))
    done <<EOF
$1
EOF
    if [ "$_n" -gt 0 ]; then CAP_STORAGE=available; fi
}

# Memory modules read as MB<US>MAKER<US>PART<US>SPEED<US>CONFIGURED<US>TYPE<US>FORM, one per line.
emit_memory_rows() {
    _n=0
    _configured=no
    while IFS=$US read -r _mb _maker _part _speed _cspeed _type _form; do
        [ -n "$_mb" ] || continue
        if [ "$_n" -ge "$MAX_ITEMS" ]; then
            note "Only the first $MAX_ITEMS memory modules are listed."
            break
        fi
        emit "memory.$_n.size_mb" "$_mb"
        emit "memory.$_n.maker" "$_maker"
        emit "memory.$_n.part" "$_part"
        emit "memory.$_n.speed_mts" "$_speed"
        emit "memory.$_n.configured_speed_mts" "$_cspeed"
        emit "memory.$_n.type" "$_type"
        emit "memory.$_n.form" "$_form"
        [ -z "$_cspeed" ] || _configured=yes
        _n=$((_n + 1))
    done <<EOF
$1
EOF
    if [ "$_n" -gt 0 ]; then CAP_MEMORY_MODULES=available; fi
    if [ "$_configured" = yes ]; then CAP_CONFIGURED_MEMORY=available; fi
}

# dmidecode -t 17, by exact key. "Serial Number", "Asset Tag" and the locators are never kept.
dmidecode_rows() {
    awk -v us="$US" '
        function flush() {
            if (inside && size > 0) print size us maker us part us speed us configured us type us form
            inside = 0; size = 0; maker = ""; part = ""; speed = ""; configured = ""; type = ""; form = ""
        }
        function mb(text, parts) {
            split(text, parts, " ")
            if (parts[2] == "TB") return parts[1] * 1048576
            if (parts[2] == "GB") return parts[1] * 1024
            if (parts[2] == "MB") return parts[1] + 0
            return 0
        }
        function number(text) { return (text ~ /^[0-9]+/) ? text + 0 : "" }
        /^Memory Device/ { flush(); inside = 1; next }
        /^[^ \t]/ { flush(); next }
        inside {
            at = index($0, ":")
            if (at == 0) next
            key = substr($0, 1, at - 1)
            value = substr($0, at + 1)
            gsub(/^[ \t]+|[ \t]+$/, "", key)
            gsub(/^[ \t]+|[ \t]+$/, "", value)
            if (key == "Size") size = mb(value)
            else if (key == "Manufacturer") maker = value
            else if (key == "Part Number") part = value
            else if (key == "Speed") speed = number(value)
            else if (key == "Configured Memory Speed" || key == "Configured Clock Speed") configured = number(value)
            else if (key == "Type") type = value
            else if (key == "Form Factor") form = value
        }
        END { flush() }'
}

# dmidecode runs only when the member asked for it with --with-root, and only for memory modules.
memory_rows_with_root() {
    have dmidecode || return 0
    if [ "$(id -u)" = 0 ]; then
        dmidecode -t 17 2>/dev/null | dmidecode_rows
    elif have sudo; then
        sudo dmidecode -t 17 2>/dev/null | dmidecode_rows
    elif have doas; then
        doas dmidecode -t 17 2>/dev/null | dmidecode_rows
    fi
}

# Where no source without root named the modules: --with-root, or say what it would take.
memory_modules_through_dmidecode() {
    _rows=''
    if [ "$WITH_ROOT" = yes ]; then
        _rows=$(memory_rows_with_root)
        [ -n "$_rows" ] || note "--with-root could not read memory modules. Is dmidecode installed, and may you use sudo or doas?"
    fi
    if [ -n "$_rows" ]; then
        emit_memory_rows "$_rows"
        return 0
    fi
    CAP_MEMORY_MODULES=needs-root
    if [ "$WITH_ROOT" = no ]; then
        if have dmidecode; then
            note "Memory modules need root here. To add them, run: sh mysetup-scan.sh --with-root"
        else
            note "Memory modules need root and dmidecode here. To add them, run: $(install_command dmidecode)"
            note "Then run: sh mysetup-scan.sh --with-root"
        fi
    fi
}

# ---------------------------------------------------------------------------------------------
# Linux. Tested on real machines; see the README.
# ---------------------------------------------------------------------------------------------

environment_linux() {
    case $(first_line "$ROOT/proc/sys/kernel/osrelease" | awk '{ print tolower($0) }') in
        *microsoft* | *wsl*)
            echo wsl
            return 0
            ;;
    esac
    if have systemd-detect-virt; then
        case $(systemd-detect-virt 2>/dev/null) in
            none)
                echo physical
                return 0
                ;;
            wsl)
                echo wsl
                return 0
                ;;
            lxc | lxc-libvirt | systemd-nspawn | docker | podman | rkt | openvz | proot | pouch | container-other)
                echo container
                return 0
                ;;
            '') ;;
            *)
                echo virtual_machine
                return 0
                ;;
        esac
    fi
    if [ -e "$ROOT/run/systemd/container" ] || [ -e "$ROOT/.dockerenv" ] || [ -e "$ROOT/run/.containerenv" ]; then
        echo container
        return 0
    fi
    if [ -r "$ROOT/proc/cpuinfo" ]; then
        _flags=$(field ':' flags <"$ROOT/proc/cpuinfo")
        case " $_flags " in
            *' hypervisor '*)
                echo virtual_machine
                return 0
                ;;
        esac
        if [ -n "$_flags" ]; then
            echo physical
            return 0
        fi
    fi
    _board=$(first_line "$ROOT/proc/device-tree/model")
    case $_board in
        '' | *[Vv]irt* | *KVM* | *QEMU*) echo unknown ;;
        *) echo physical ;;
    esac
}

read_cpu_linux() {
    _vendor=''
    _model=''
    if have lscpu; then
        _lscpu=$(lscpu 2>/dev/null)
        _vendor=$(printf '%s\n' "$_lscpu" | field ':' 'Vendor ID')
        _model=$(printf '%s\n' "$_lscpu" | field ':' 'Model name')
    fi
    if [ -r "$ROOT/proc/cpuinfo" ]; then
        [ -n "$_vendor" ] || _vendor=$(field ':' vendor_id <"$ROOT/proc/cpuinfo")
        [ -n "$_model" ] || _model=$(field ':' 'model name' <"$ROOT/proc/cpuinfo")
        [ -n "$_model" ] || _model=$(field ':' Processor <"$ROOT/proc/cpuinfo")
    fi

    # Each logical CPU's package and core, counted once each. This holds for hybrid CPUs, whose
    # "Core(s) per socket" in lscpu is not the number of cores.
    _threads=0
    _pairs=''
    for _cpu in "$ROOT"/sys/devices/system/cpu/cpu[0-9]*; do
        [ -d "$_cpu" ] || continue
        _threads=$((_threads + 1))
        _package=$(first_line "$_cpu/topology/physical_package_id")
        _core=$(first_line "$_cpu/topology/core_id")
        [ -n "$_core" ] && _pairs="$_pairs$_package:$_core
"
    done
    _cores=$(printf '%s' "$_pairs" | awk 'NF && !seen[$0]++ { n++ } END { if (n) print n }')
    if [ "$_threads" -eq 0 ] && [ -r "$ROOT/proc/cpuinfo" ]; then
        _threads=$(awk '$1 == "processor" { n++ } END { print n + 0 }' "$ROOT/proc/cpuinfo")
    fi

    emit cpu.vendor "$(normal_cpu_vendor "$_vendor")"
    emit cpu.model "$_model"
    emit cpu.cores "$_cores"
    emit cpu.threads "$_threads"
    if [ -n "$_model" ] || [ "$_threads" -gt 0 ]; then CAP_CPU=available; fi
}

read_gpu_linux() {
    _n=0
    for _dev in "$ROOT"/sys/bus/pci/devices/*; do
        [ -r "$_dev/class" ] || continue
        case $(first_line "$_dev/class") in
            0x03*) ;;
            *) continue ;;
        esac
        if [ "$_n" -ge "$MAX_ITEMS" ]; then
            note "Only the first $MAX_ITEMS graphics adapters are listed."
            break
        fi
        _vendor=$(first_line "$_dev/vendor")
        _device=$(first_line "$_dev/device")
        _svendor=$(first_line "$_dev/subsystem_vendor")
        _sdevice=$(first_line "$_dev/subsystem_device")
        emit "gpu.$_n.pci" "${_vendor#0x}:${_device#0x}"
        emit "gpu.$_n.subsystem" "${_svendor#0x}:${_sdevice#0x}"
        if have lspci; then
            # The names lspci knows, by exact key; the trailing [id] is already in the report.
            _block=$(lspci -vmm -nn -s "${_dev##*/}" 2>/dev/null)
            _name=$(printf '%s\n' "$_block" | field ':' Vendor | awk '{ sub(/ *\[[0-9a-f]+\]$/, ""); print }')
            emit "gpu.$_n.vendor" "$_name"
            _name=$(printf '%s\n' "$_block" | field ':' Device | awk '{ sub(/ *\[[0-9a-f]+\]$/, ""); print }')
            case $_name in
                'Device '????) ;;
                *) emit "gpu.$_n.model" "$_name" ;;
            esac
        fi
        _n=$((_n + 1))
    done
    if [ "$_n" -gt 0 ] && ! have lspci; then
        note "Graphics are listed by PCI ID only. To add their names, run: $(install_command pciutils)"
    fi

    # Graphics that are not on PCI, as on many ARM boards: what the device tree calls them.
    for _card in "$ROOT"/sys/class/drm/card[0-9] "$ROOT"/sys/class/drm/card[0-9][0-9]; do
        [ -r "$_card/device/uevent" ] || continue
        [ -z "$(field '=' PCI_ID <"$_card/device/uevent")" ] || continue
        _name=$(field '=' OF_COMPATIBLE_0 <"$_card/device/uevent")
        [ -n "$_name" ] || continue
        emit "gpu.$_n.model" "$_name"
        _n=$((_n + 1))
    done

    if [ "$_n" -gt 0 ]; then CAP_GPU=available; fi
}

# A chassis type's number as SMBIOS names it (DSP0134 7.4.1); "Other" and "Unknown" give nothing. The top bit is a lock.
chassis_name() {
    case $1 in '' | *[!0-9]*) return 0 ;; esac
    case $(($1 % 128)) in
        3) echo desktop ;; 4) echo low-profile-desktop ;; 5) echo pizza-box ;; 6) echo mini-tower ;; 7) echo tower ;;
        8) echo portable ;; 9) echo laptop ;; 10) echo notebook ;; 11) echo hand-held ;; 12) echo docking-station ;;
        13) echo all-in-one ;; 14) echo sub-notebook ;; 15) echo space-saving ;; 16) echo lunch-box ;;
        17) echo main-server-chassis ;; 18) echo expansion-chassis ;; 19) echo sub-chassis ;; 20) echo bus-expansion-chassis ;;
        21) echo peripheral-chassis ;; 22) echo raid-chassis ;; 23) echo rack-mount-chassis ;; 24) echo sealed-case-pc ;;
        25) echo multi-system-chassis ;; 26) echo compact-pci ;; 27) echo advanced-tca ;; 28) echo blade ;; 29) echo blade-enclosure ;;
        30) echo tablet ;; 31) echo convertible ;; 32) echo detachable ;; 33) echo iot-gateway ;; 34) echo embedded-pc ;;
        35) echo mini-pc ;; 36) echo stick-pc ;;
    esac
}

# The computer's own model, as its maker names it, by file name: product_serial, product_uuid, chassis_serial and
# chassis_asset_tag beside them are never read (the operator, 2026-11-13: "we need to match models").
read_system_linux() {
    _dmi=$ROOT/sys/class/dmi/id
    emit system.vendor "$(first_line "$_dmi/sys_vendor")"
    emit system.model "$(first_line "$_dmi/product_name")"
    emit system.family "$(first_line "$_dmi/product_family")"
    emit system.version "$(first_line "$_dmi/product_version")"
    emit system.sku "$(first_line "$_dmi/product_sku")"
    emit system.chassis "$(chassis_name "$(first_line "$_dmi/chassis_type")")"
}

read_motherboard_linux() {
    _vendor=$(first_line "$ROOT/sys/class/dmi/id/board_vendor")
    _model=$(first_line "$ROOT/sys/class/dmi/id/board_name")
    if [ -z "$_model" ]; then
        _model=$(first_line "$ROOT/proc/device-tree/model")
    fi
    _printed=$(emit motherboard.vendor "$_vendor"; emit motherboard.model "$_model")
    if [ -n "$_printed" ]; then
        printf '%s\n' "$_printed"
        CAP_MOTHERBOARD=available
    fi
}

# udev's copy of the memory tables, which any user may read on systemd hosts. It also holds serial
# numbers and asset tags, so only the fields named here are kept.
udev_memory_rows() {
    have udevadm || return 0
    udevadm info -q property -p /devices/virtual/dmi/id 2>/dev/null | awk -v us="$US" '
        /^MEMORY_DEVICE_[0-9]+_[A-Z_]+=/ {
            at = index($0, "=")
            key = substr($0, 15, at - 15)
            value = substr($0, at + 1)
            split(key, parts, "_")
            n = parts[1] + 0
            field = substr(key, length(parts[1]) + 2)
            if (n > last) last = n
            if (field == "SIZE" || field == "PRESENT" || field == "MANUFACTURER" || field == "PART_NUMBER" || \
                field == "SPEED_MTS" || field == "CONFIGURED_SPEED_MTS" || field == "TYPE" || field == "FORM_FACTOR")
                device[n, field] = value
        }
        END {
            for (n = 0; n <= last; n++) {
                size = device[n, "SIZE"] + 0
                if (size <= 0 || device[n, "PRESENT"] == "0") continue
                printf "%d%s%s%s%s%s%s%s%s%s%s%s%s\n", size / 1048576, us, device[n, "MANUFACTURER"], us, \
                    device[n, "PART_NUMBER"], us, device[n, "SPEED_MTS"], us, device[n, "CONFIGURED_SPEED_MTS"], us, \
                    device[n, "TYPE"], us, device[n, "FORM_FACTOR"]
            }
        }'
}

read_memory_linux() {
    if [ -r "$ROOT/proc/meminfo" ]; then
        emit memory.total_mb "$(awk '$1 == "MemTotal:" { printf "%d", $2 / 1024; exit }' "$ROOT/proc/meminfo")"
    fi
    _rows=$(udev_memory_rows)
    if [ -n "$_rows" ]; then
        emit_memory_rows "$_rows"
    elif [ "$environment" != container ] && [ -e "$ROOT/sys/class/dmi/id" ]; then
        memory_modules_through_dmidecode
    else
        CAP_MEMORY_MODULES=not-exposed
    fi
}

# Whole physical disks only. Virtual and partition-like devices are not parts of the build.
linux_disk_name_wanted() {
    case $1 in
        loop* | ram* | zram* | zd* | nbd* | md* | dm-* | sr* | fd* | mtdblock* | drbd* | rbd* | bcache* | nullb*) return 1 ;;
        *boot[0-9] | *rpmb) return 1 ;;
    esac
    return 0
}

linux_interface_from_name() {
    case $1 in
        nvme*) echo nvme ;;
        mmcblk*) echo mmc ;;
        vd*) echo virtio ;;
        xvd*) echo xen ;;
    esac
}

# Columns are asked for by name, and SERIAL, WWN and the like are not among them.
lsblk_storage_rows() {
    have lsblk || return 0
    _lines=$(lsblk -d -b -n -P -o NAME,TYPE,MODEL,SIZE,TRAN,ROTA 2>/dev/null) || return 0
    while IFS= read -r _line; do
        [ -n "$_line" ] || continue
        _fields=$(printf '%s\n' "$_line" | awk -v us="$US" '
            function pick(key) {
                if (match(line, "(^| )" key "=\"[^\"]*\"")) {
                    text = substr(line, RSTART, RLENGTH)
                    sub(/^ /, "", text)
                    return substr(text, length(key) + 3, length(text) - length(key) - 3)
                }
                return ""
            }
            { line = $0; print pick("NAME") us pick("TYPE") us pick("MODEL") us pick("SIZE") us pick("TRAN") us pick("ROTA") }')
        IFS=$US read -r _name _type _model _bytes _tran _rota <<EOF
$_fields
EOF
        [ "$_type" = disk ] || continue
        linux_disk_name_wanted "$_name" || continue
        case $_bytes in '' | 0) continue ;; esac
        [ -n "$_model" ] || _model=$(first_line "$ROOT/sys/block/$_name/device/name")
        _interface=$_tran
        [ -n "$_interface" ] || _interface=$(linux_interface_from_name "$_name")
        _media=''
        case $_interface in
            usb | virtio | xen | '') ;;
            *)
                [ "$_rota" != 0 ] || _media=ssd
                [ "$_rota" != 1 ] || _media=hdd
                ;;
        esac
        printf '%s%s%s%s%s%s%s\n' "$_model" "$US" "$_bytes" "$US" "$_interface" "$US" "$_media"
    done <<EOF
$_lines
EOF
}

sys_storage_rows() {
    for _block in "$ROOT"/sys/block/*; do
        [ -d "$_block" ] || continue
        _name=${_block##*/}
        linux_disk_name_wanted "$_name" || continue
        _sectors=$(first_line "$_block/size")
        case $_sectors in '' | 0 | *[!0-9]*) continue ;; esac
        _bytes=$(awk -v sectors="$_sectors" 'BEGIN { printf "%.0f", sectors * 512 }')
        _model=$(first_line "$_block/device/model")
        [ -n "$_model" ] || _model=$(first_line "$_block/device/name")
        _interface=$(linux_interface_from_name "$_name")
        if [ -z "$_interface" ]; then
            case $(cd -P "$_block" 2>/dev/null && pwd -P) in
                */usb*) _interface=usb ;;
                */ata*) _interface=sata ;;
            esac
        fi
        _media=''
        case $_interface in
            usb | virtio | xen | '') ;;
            *)
                _rota=$(first_line "$_block/queue/rotational")
                [ "$_rota" != 0 ] || _media=ssd
                [ "$_rota" != 1 ] || _media=hdd
                ;;
        esac
        printf '%s%s%s%s%s%s%s\n' "$_model" "$US" "$_bytes" "$US" "$_interface" "$US" "$_media"
    done
}

read_storage_linux() {
    _rows=$(lsblk_storage_rows)
    [ -n "$_rows" ] || _rows=$(sys_storage_rows)
    emit_storage_rows "$_rows"
}

read_os_linux() {
    _file=$ROOT/etc/os-release
    [ -r "$_file" ] || _file=$ROOT/usr/lib/os-release
    [ -r "$_file" ] || return 0
    # Read as text, never run: this file is shell syntax, and sourcing it would execute it.
    _id=$(field '=' ID <"$_file" | tr -d '"'"'")
    _version=$(field '=' VERSION_ID <"$_file" | tr -d '"'"'")
    emit os.name "$_id"
    emit os.version "$_version"
}

read_linux() {
    read_cpu_linux
    read_gpu_linux
    read_motherboard_linux
    read_memory_linux
    read_storage_linux
}

# ---------------------------------------------------------------------------------------------
# macOS. Not yet tested on a real Mac: it follows Apple's tools as documented.
# ---------------------------------------------------------------------------------------------

read_macos() {
    _model=$(sysctl -n machdep.cpu.brand_string 2>/dev/null)
    case $_model in
        Apple*) emit cpu.vendor Apple ;;
        *Intel*) emit cpu.vendor Intel ;;
    esac
    emit cpu.model "$_model"
    emit cpu.cores "$(sysctl -n hw.physicalcpu 2>/dev/null)"
    emit cpu.threads "$(sysctl -n hw.logicalcpu 2>/dev/null)"
    [ -z "$_model" ] || CAP_CPU=available

    # Only these three keys; the "Displays" entries below them hold serial numbers.
    _rows=$(system_profiler SPDisplaysDataType 2>/dev/null | awk -v us="$US" '
        function flush() {
            if (model != "") print model us name us vendor us device
            model = ""; name = ""; vendor = ""; device = ""
        }
        {
            at = index($0, ":")
            if (at == 0) next
            key = substr($0, 1, at - 1)
            value = substr($0, at + 1)
            gsub(/^[ \t]+|[ \t]+$/, "", key)
            gsub(/^[ \t]+|[ \t]+$/, "", value)
            if (key == "Chipset Model") { flush(); model = value }
            else if (key == "Vendor") {
                name = value
                if (match(value, /\(0x[0-9a-f]+\)/)) {
                    vendor = substr(value, RSTART + 3, RLENGTH - 4)
                    sub(/ *\(0x[0-9a-f]+\)/, "", name)
                }
            }
            else if (key == "Device ID") { device = value; sub(/^0x/, "", device) }
        }
        END { flush() }')
    _n=0
    while IFS=$US read -r _model _name _vendor _device; do
        [ -n "$_model" ] || continue
        emit "gpu.$_n.model" "$_model"
        case $_name in
            sppci_vendor_*) emit "gpu.$_n.vendor" "${_name#sppci_vendor_}" ;;
            *) emit "gpu.$_n.vendor" "$_name" ;;
        esac
        if [ -n "$_vendor" ] && [ -n "$_device" ]; then emit "gpu.$_n.pci" "$_vendor:$_device"; fi
        _n=$((_n + 1))
    done <<EOF
$_rows
EOF
    [ "$_n" -eq 0 ] || CAP_GPU=available

    _hw=$(sysctl -n hw.model 2>/dev/null)
    if [ -n "$_hw" ]; then
        emit motherboard.vendor Apple
        emit motherboard.model "$_hw"
        CAP_MOTHERBOARD=available
    fi

    _bytes=$(sysctl -n hw.memsize 2>/dev/null)
    case $_bytes in
        '' | *[!0-9]*) ;;
        *) emit memory.total_mb "$(awk -v bytes="$_bytes" 'BEGIN { printf "%d", bytes / 1048576 }')" ;;
    esac
    # By exact key; "Serial Number" is not kept. Empty slots say "Empty" and are skipped.
    _rows=$(system_profiler SPMemoryDataType 2>/dev/null | awk -v us="$US" '
        function flush() {
            if (size > 0) print size us maker us part us speed us "" us type us ""
            size = 0; maker = ""; part = ""; speed = ""; type = ""
        }
        function mb(text, parts) {
            split(text, parts, " ")
            if (parts[2] == "TB") return parts[1] * 1048576
            if (parts[2] == "GB") return parts[1] * 1024
            if (parts[2] == "MB") return parts[1] + 0
            return 0
        }
        {
            at = index($0, ":")
            if (at == 0) next
            key = substr($0, 1, at - 1)
            value = substr($0, at + 1)
            gsub(/^[ \t]+|[ \t]+$/, "", key)
            gsub(/^[ \t]+|[ \t]+$/, "", value)
            if (key == "Size" || (key == "Memory" && value != "")) { flush(); size = mb(value) }
            else if (key == "Type") type = value
            else if (key == "Speed") speed = (value ~ /^[0-9]+/) ? value + 0 : ""
            else if (key == "Manufacturer") maker = value
            else if (key == "Part Number") part = value
        }
        END { flush() }')
    emit_memory_rows "$_rows"
    [ "$CAP_MEMORY_MODULES" = available ] || CAP_MEMORY_MODULES=not-exposed
    [ "$CAP_CONFIGURED_MEMORY" = available ] || CAP_CONFIGURED_MEMORY=not-exposed

    # "diskutil info" by exact key, for the disks it calls physical.
    _rows=''
    for _disk in $(diskutil list physical 2>/dev/null | awk '/^\/dev\/disk[0-9]+ / { print $1 }'); do
        _info=$(diskutil info "$_disk" 2>/dev/null)
        _model=$(printf '%s\n' "$_info" | field ':' 'Device / Media Name')
        _bytes=$(printf '%s\n' "$_info" | field ':' 'Disk Size' | awk 'match($0, /\([0-9]+ Bytes\)/) { print substr($0, RSTART + 1, RLENGTH - 8) }')
        case $(printf '%s\n' "$_info" | field ':' Protocol) in
            'Apple Fabric') _interface=apple-fabric ;;
            PCI-Express) _interface=nvme ;;
            SATA) _interface=sata ;;
            USB) _interface=usb ;;
            Thunderbolt) _interface=thunderbolt ;;
            *) _interface='' ;;
        esac
        case $(printf '%s\n' "$_info" | field ':' 'Solid State') in
            Yes) _media=ssd ;;
            No) _media=hdd ;;
            *) _media='' ;;
        esac
        _rows="$_rows$_model$US$_bytes$US$_interface$US$_media
"
    done
    emit_storage_rows "$_rows"
}

environment_macos() {
    case $(sysctl -n kern.hv_vmm_present 2>/dev/null) in
        1) echo virtual_machine ;;
        0) echo physical ;;
        *) echo unknown ;;
    esac
}

# ---------------------------------------------------------------------------------------------
# FreeBSD. Not yet tested on a real machine: it follows the system's tools as documented.
# ---------------------------------------------------------------------------------------------

read_freebsd() {
    _model=$(sysctl -n hw.model 2>/dev/null)
    case $_model in
        *Intel*) emit cpu.vendor Intel ;;
        *AMD*) emit cpu.vendor AMD ;;
    esac
    emit cpu.model "$_model"
    emit cpu.cores "$(sysctl -n kern.smp.cores 2>/dev/null)"
    emit cpu.threads "$(sysctl -n hw.ncpu 2>/dev/null)"
    [ -z "$_model" ] || CAP_CPU=available

    # Display-class devices from "pciconf -l": vendor=/device= here, chip=/card= on older systems.
    _rows=$(pciconf -l 2>/dev/null | awk -v us="$US" '
        function pick(key) {
            return match($0, key "=0x[0-9a-f]+") ? substr($0, RSTART + length(key) + 3, RLENGTH - length(key) - 3) : ""
        }
        /class=0x03/ {
            vendor = pick("vendor"); device = pick("device"); svendor = pick("subvendor"); sdevice = pick("subdevice")
            chip = pick("chip"); card = pick("card")
            if (vendor == "" && length(chip) == 8) { device = substr(chip, 1, 4); vendor = substr(chip, 5) }
            if (svendor == "" && length(card) == 8) { sdevice = substr(card, 1, 4); svendor = substr(card, 5) }
            print vendor us device us svendor us sdevice
        }')
    _n=0
    while IFS=$US read -r _vendor _device _svendor _sdevice; do
        [ -n "$_vendor" ] || continue
        emit "gpu.$_n.pci" "$_vendor:$_device"
        emit "gpu.$_n.subsystem" "$_svendor:$_sdevice"
        _n=$((_n + 1))
    done <<EOF
$_rows
EOF
    [ "$_n" -eq 0 ] || CAP_GPU=available

    _printed=$(emit motherboard.vendor "$(kenv -q smbios.planar.maker 2>/dev/null)"
        emit motherboard.model "$(kenv -q smbios.planar.product 2>/dev/null)")
    if [ -n "$_printed" ]; then
        printf '%s\n' "$_printed"
        CAP_MOTHERBOARD=available
    fi

    _bytes=$(sysctl -n hw.physmem 2>/dev/null)
    case $_bytes in
        '' | *[!0-9]*) ;;
        *) emit memory.total_mb "$(awk -v bytes="$_bytes" 'BEGIN { printf "%d", bytes / 1048576 }')" ;;
    esac
    memory_modules_through_dmidecode

    # "geom disk list" by exact key; "ident" (the serial number) and "lunid" are not kept.
    _rows=$(geom disk list 2>/dev/null | awk -v us="$US" '
        function flush() {
            if (name != "" && name !~ /^cd/) print name us model us bytes us rotation
            name = ""; model = ""; bytes = ""; rotation = ""
        }
        {
            at = index($0, ":")
            if (at == 0) next
            key = substr($0, 1, at - 1)
            value = substr($0, at + 1)
            gsub(/^[ \t]+|[ \t]+$/, "", key)
            gsub(/^[ \t]+|[ \t]+$/, "", value)
            if (key == "Geom name") { flush(); name = value }
            else if (key == "Mediasize") { split(value, parts, " "); bytes = parts[1] }
            else if (key == "descr") model = value
            else if (key == "rotationrate") rotation = value
        }
        END { flush() }')
    _disks=''
    while IFS=$US read -r _name _model _bytes _rotation; do
        [ -n "$_name" ] || continue
        case $_name in
            ada*) _interface=sata ;;
            nvd* | nda*) _interface=nvme ;;
            mmcsd*) _interface=mmc ;;
            vtbd*) _interface=virtio ;;
            *) _interface='' ;;
        esac
        _media=''
        case $_rotation in
            0) _media=ssd ;;
            [1-9]*) _media=hdd ;;
        esac
        _disks="$_disks$_model$US$_bytes$US$_interface$US$_media
"
    done <<EOF
$_rows
EOF
    emit_storage_rows "$_disks"
}

environment_freebsd() {
    if [ "$(sysctl -n security.jail.jailed 2>/dev/null)" = 1 ]; then
        echo container
        return 0
    fi
    case $(sysctl -n kern.vm_guest 2>/dev/null) in
        none) echo physical ;;
        '') echo unknown ;;
        *) echo virtual_machine ;;
    esac
}

# ---------------------------------------------------------------------------------------------
# The other BSDs, illumos and anything else. Not yet tested: the basics, and what they cannot read
# is reported as such rather than guessed.
# ---------------------------------------------------------------------------------------------

# A Mac is its model ("MacBookPro18,3"), which the board line also names.
read_system_macos() {
    _hw=$(sysctl -n hw.model 2>/dev/null)
    if [ -n "$_hw" ]; then
        emit system.vendor Apple
        emit system.model "$_hw"
    fi
}

# By name: smbios.system.serial and smbios.system.uuid are never read. FreeBSD hands over no chassis type.
read_system_freebsd() {
    emit system.vendor "$(kenv -q smbios.system.maker 2>/dev/null)"
    emit system.model "$(kenv -q smbios.system.product 2>/dev/null)"
    emit system.family "$(kenv -q smbios.system.family 2>/dev/null)"
    emit system.version "$(kenv -q smbios.system.version 2>/dev/null)"
    emit system.sku "$(kenv -q smbios.system.sku 2>/dev/null)"
}

# $1 the computer's maker, $2 its model: sysctl names.
read_system_sysctl() {
    emit system.vendor "$(sysctl -n "$1" 2>/dev/null)"
    emit system.model "$(sysctl -n "$2" 2>/dev/null)"
}

# By exact key; "Serial Number", "UUID" and the chassis's "Asset Tag" are not kept. "Chassis Type: 0xa (notebook)".
read_system_illumos() {
    _system=$(smbios -t SMB_TYPE_SYSTEM 2>/dev/null)
    emit system.vendor "$(printf '%s\n' "$_system" | field ':' Manufacturer)"
    emit system.model "$(printf '%s\n' "$_system" | field ':' Product)"
    emit system.family "$(printf '%s\n' "$_system" | field ':' Family)"
    emit system.version "$(printf '%s\n' "$_system" | field ':' Version)"
    emit system.sku "$(printf '%s\n' "$_system" | field ':' 'SKU Number')"
    _type=$(smbios -t SMB_TYPE_CHASSIS 2>/dev/null | field ':' 'Chassis Type' | awk '{ print $1 }')
    case $_type in 0x[0-9a-fA-F]*) emit system.chassis "$(chassis_name "$((_type))")" ;; esac
}

read_sysctl_basics() {
    # $1 CPU model, $2 thread count, $3 memory bytes, $4 board vendor, $5 board model: sysctl names.
    _model=$(sysctl -n "$1" 2>/dev/null)
    emit cpu.model "$_model"
    emit cpu.threads "$(sysctl -n "$2" 2>/dev/null)"
    [ -z "$_model" ] || CAP_CPU=available
    _bytes=$(sysctl -n "$3" 2>/dev/null)
    case $_bytes in
        '' | *[!0-9]*) ;;
        *) emit memory.total_mb "$(awk -v bytes="$_bytes" 'BEGIN { printf "%d", bytes / 1048576 }')" ;;
    esac
    _printed=$(emit motherboard.vendor "$(sysctl -n "$4" 2>/dev/null)"
        emit motherboard.model "$(sysctl -n "$5" 2>/dev/null)")
    if [ -n "$_printed" ]; then
        printf '%s\n' "$_printed"
        CAP_MOTHERBOARD=available
    fi
    memory_modules_through_dmidecode
}

read_illumos() {
    _model=$(psrinfo -pv 2>/dev/null | awk 'NF { line = $0 } END { gsub(/^[ \t]+/, "", line); print line }')
    emit cpu.model "$_model"
    emit cpu.threads "$(psrinfo 2>/dev/null | awk 'END { print NR }')"
    [ -z "$_model" ] || CAP_CPU=available
    emit memory.total_mb "$(prtconf -m 2>/dev/null)"
    # By exact key; "Serial Number" is not kept. /dev/smbios may need root, and then this is empty.
    _board=$(smbios -t SMB_TYPE_BASEBOARD 2>/dev/null)
    _printed=$(emit motherboard.vendor "$(printf '%s\n' "$_board" | field ':' Manufacturer)"
        emit motherboard.model "$(printf '%s\n' "$_board" | field ':' Product)")
    if [ -n "$_printed" ]; then
        printf '%s\n' "$_printed"
        CAP_MOTHERBOARD=available
    fi
    memory_modules_through_dmidecode
}

read_other() {
    if [ -r "$ROOT/proc/cpuinfo" ]; then
        _model=$(field ':' 'model name' <"$ROOT/proc/cpuinfo")
        emit cpu.model "$_model"
        [ -z "$_model" ] || CAP_CPU=available
    fi
}

# ---------------------------------------------------------------------------------------------
# The report
# ---------------------------------------------------------------------------------------------

usage() {
    printf '%s\n' \
        "MySetup scan $VERSION: prints a report of this computer's hardware to paste on your build page." \
        '' \
        'Usage: sh mysetup-scan.sh [--with-root] [--help] [--version]' \
        '' \
        '  --with-root  Use dmidecode through sudo or doas to read memory modules, where they cannot be' \
        '               read without root. It asks for nothing else and is used for nothing else.' \
        '' \
        'It only reads: no network, no files written, nothing installed.'
}

main() {
    for _arg in "$@"; do
        case $_arg in
            --with-root) WITH_ROOT=yes ;;
            --help | -h)
                usage
                return 0
                ;;
            --version)
                echo "mysetup-scan $VERSION"
                return 0
                ;;
            *)
                usage >&2
                return 2
                ;;
        esac
    done

    for _tool in awk tr uname id; do
        if ! have "$_tool"; then
            printf 'mysetup-scan: this system has no %s, which the scan needs.\n' "$_tool" >&2
            return 3
        fi
    done

    CAP_CPU=no-source
    CAP_GPU=no-source
    CAP_MOTHERBOARD=no-source
    CAP_MEMORY_MODULES=no-source
    CAP_CONFIGURED_MEMORY=no-source
    CAP_STORAGE=no-source

    _system=$(uname -s)
    _architecture=$(normal_architecture "$(uname -m)")
    case $_system in
        Linux)
            family=linux
            environment=$(environment_linux)
            ;;
        Darwin)
            family=macos
            environment=$(environment_macos)
            if [ "$(sysctl -n hw.optional.arm64 2>/dev/null)" = 1 ]; then _architecture=arm64; fi
            ;;
        FreeBSD)
            family=freebsd
            environment=$(environment_freebsd)
            ;;
        OpenBSD)
            family=openbsd
            environment=unknown
            ;;
        NetBSD)
            family=netbsd
            environment=unknown
            ;;
        SunOS)
            family=illumos
            environment=unknown
            ;;
        CYGWIN* | MINGW* | MSYS*)
            echo 'This shell runs on Windows. Please use mysetup-scan.ps1 in PowerShell instead. Nothing was reported.' >&2
            return 4
            ;;
        *)
            family=other
            environment=unknown
            ;;
    esac

    if [ "$environment" = wsl ]; then
        echo 'This is WSL, which sees virtual hardware. Please run mysetup-scan.ps1 in Windows instead. Nothing was reported.' >&2
        return 4
    fi

    # A system counts as tested once a real machine has passed; see the README.
    tested=no
    case $family/$_architecture in
        linux/x86-64)
            if have udevadm; then tested=yes; fi
            ;;
    esac

    {
        echo "MySetup scan $VERSION. It reads hardware names and sends nothing anywhere."
        echo 'Copy everything from "mysetup-report 1" to "mysetup-report-end" and paste it on your build page.'
        echo
    } >&2

    echo 'mysetup-report 1'
    emit source "script-sh/$VERSION"
    emit tested "$tested"
    emit os.family "$family"
    case $family in
        linux) read_os_linux ;;
        macos)
            emit os.name macos
            emit os.version "$(sw_vers -productVersion 2>/dev/null)"
            ;;
        freebsd)
            emit os.name freebsd
            emit os.version "$(freebsd-version -u 2>/dev/null)"
            ;;
        openbsd | netbsd | illumos)
            emit os.name "$family"
            emit os.version "$(uname -r)"
            ;;
    esac
    emit os.architecture "$_architecture"
    emit environment "$environment"

    # The computer's model only on a physical computer, whose firmware its maker wrote. A virtual machine's can be set to
    # anything, its host's name among them, so it is never read there; nor where the script cannot tell (unknown).
    if [ "$environment" = physical ]; then
        case $family in
            linux) read_system_linux ;;
            macos) read_system_macos ;;
            freebsd) read_system_freebsd ;;
            openbsd) read_system_sysctl hw.vendor hw.product ;;
            netbsd) read_system_sysctl machdep.dmi.system-vendor machdep.dmi.system-product ;;
            illumos) read_system_illumos ;;
        esac
    fi

    case $family in
        linux) read_linux ;;
        macos) read_macos ;;
        freebsd) read_freebsd ;;
        openbsd) read_sysctl_basics hw.model hw.ncpuonline hw.physmem hw.vendor hw.product ;;
        netbsd) read_sysctl_basics machdep.cpu_brand hw.ncpuonline hw.physmem64 machdep.dmi.board-vendor machdep.dmi.board-product ;;
        illumos) read_illumos ;;
        *) read_other ;;
    esac

    capability cpu "$CAP_CPU"
    capability gpu "$CAP_GPU"
    capability motherboard "$CAP_MOTHERBOARD"
    capability memory_modules "$CAP_MEMORY_MODULES"
    capability configured_memory "$CAP_CONFIGURED_MEMORY"
    capability storage "$CAP_STORAGE"
    capability displays not-collected
    echo 'mysetup-report-end'

    if [ "$tested" = no ]; then
        note "Untested on this system ($family/$_architecture). Parts may be missing or wrong, so please check the report before you paste it."
    fi
    case $environment in
        virtual_machine) note "This looks like a virtual machine, so the report shows what it was given, not the real computer." ;;
        container) note "This looks like a container, so the report shows what it was given, not the real computer." ;;
    esac
    if [ -n "$NOTES" ]; then
        echo >&2
        printf '%s' "$NOTES" >&2
    fi
    return 0
}

main "$@"
