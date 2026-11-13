<#
    MySetup scan: prints a report of this computer's hardware, for you to paste on your MySetup
    build page. The format is in docs/report-format.md.

        powershell -ExecutionPolicy Bypass -File .\mysetup-scan.ps1

    Windows blocks unsigned scripts from the internet by default. -ExecutionPolicy Bypass lifts
    that for this one run and changes no setting. No administrator rights are needed.

    It only reads. It uses no network, writes no files and installs nothing. Please read it before
    you run it; it is meant to be read.

    Windows' classes hold more than this report wants: disk serial numbers, the computer's name,
    the registered user. So every query names the exact properties it asks for, and only the fields
    named in Write-Field can be printed.
#>
#Requires -Version 5.1
[CmdletBinding()]
param(
    [switch]$Help
)

Set-StrictMode -Version 2.0
$ErrorActionPreference = 'SilentlyContinue'

$Version = '0.2.0'
$MaxItems = 64
$script:Notes = New-Object System.Collections.ArrayList

if ($Help) {
    Write-Output "MySetup scan $Version`: prints a report of this computer's hardware to paste on your build page."
    Write-Output 'It only reads: no network, no files written, nothing installed.'
    return
}

# PowerShell also runs on Linux and macOS, where these classes do not exist.
if ((Test-Path variable:IsWindows) -and -not $IsWindows) {
    [Console]::Error.WriteLine('This is for Windows. Please use mysetup-scan.sh on this system. Nothing was reported.')
    exit 4
}

$Placeholders = @(
    'to be filled by o.e.m.', 'to be filled by oem', 'default string', 'not applicable', 'not specified',
    'not available', 'n/a', 'na', 'none', 'null', 'unknown', 'undefined', 'empty', '[empty]', 'o.e.m.', 'oem',
    'system manufacturer', 'system product name', 'system version', 'baseboard product name',
    'base board product name', 'no dimm', 'nodimm', 'not installed', 'no module installed', 'dimm', 'module',
    'part number', 'serial number', '1234567890', '123456789012',
    # What firmware writes in a computer's own fields when its maker filled in nothing (system.*).
    'system sku', 'sku', 'system family', 'family', 'type1productconfigid', 'type1family', 'default', 'invalid', 'x.x'
)

# SMBIOS's chassis types (DSP0134 7.4.1) as the report names them; Other (1) and Unknown (2) give nothing.
$ChassisNames = @{
    3 = 'desktop'; 4 = 'low-profile-desktop'; 5 = 'pizza-box'; 6 = 'mini-tower'; 7 = 'tower'; 8 = 'portable'; 9 = 'laptop'
    10 = 'notebook'; 11 = 'hand-held'; 12 = 'docking-station'; 13 = 'all-in-one'; 14 = 'sub-notebook'; 15 = 'space-saving'
    16 = 'lunch-box'; 17 = 'main-server-chassis'; 18 = 'expansion-chassis'; 19 = 'sub-chassis'; 20 = 'bus-expansion-chassis'
    21 = 'peripheral-chassis'; 22 = 'raid-chassis'; 23 = 'rack-mount-chassis'; 24 = 'sealed-case-pc'; 25 = 'multi-system-chassis'
    26 = 'compact-pci'; 27 = 'advanced-tca'; 28 = 'blade'; 29 = 'blade-enclosure'; 30 = 'tablet'; 31 = 'convertible'
    32 = 'detachable'; 33 = 'iot-gateway'; 34 = 'embedded-pc'; 35 = 'mini-pc'; 36 = 'stick-pc'
}

# Printable ASCII, single spaces, at most 120 characters. Nothing for a placeholder, or for anything
# shaped like an address, an ID or a home directory: a hardware name never looks like those.
function Get-CleanText([object]$Value) {
    if ($null -eq $Value) { return '' }
    $text = (([string]$Value) -replace '[^ -~]', '' -replace '\s+', ' ').Trim()
    $low = $text.ToLowerInvariant()
    if ($text -eq '' -or $Placeholders -contains $low) { return '' }
    if ($low -match '^0+$' -or $low -match '^0x0+$' -or $low -match '^x+$' -or $low -match '^[-. ]*$') { return '' }
    if ($low -match '^(array[0-9]*_)?partnumber[0-9]*$' -or $low -match '^partnum[0-9]*$') { return '' }
    if ($low -match '^manufacturer[0-9]*$') { return '' }
    if ($low -match '[0-9a-f]{2}([:-][0-9a-f]{2}){5}') { return '' }
    if ($low -match '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}') { return '' }
    if ($low -match '[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+') { return '' }
    if ($low -match '[^ ]@[^ ]' -or $low.Contains('/home/') -or $low.Contains('/users/') -or $low.Contains('c:\')) { return '' }
    if ($text.Length -gt 120) { $text = $text.Substring(0, 120) }
    return $text
}

# The only way anything reaches the report. A field not named here stops the script.
function Write-Field([string]$Name, [object]$Value) {
    $text = ''
    if ($Name -cmatch '^(system\.(vendor|model|family|version|sku)|cpu\.(vendor|model)|motherboard\.(vendor|model)|gpu\.[0-9]+\.(vendor|model)|memory\.[0-9]+\.(maker|part)|storage\.[0-9]+\.model)$') {
        $text = Get-CleanText $Value
    }
    elseif ($Name -cmatch '^(cpu\.(cores|threads)|memory\.total_mb|memory\.[0-9]+\.(size_mb|speed_mts|configured_speed_mts)|storage\.[0-9]+\.size_bytes)$') {
        $text = [string]$Value
        if ($text -notmatch '^[1-9][0-9]*$') { return }
    }
    elseif ($Name -cmatch '^gpu\.[0-9]+\.(pci|subsystem)$') {
        $text = [string]$Value
        if ($text -cnotmatch '^[0-9a-f]{4}:[0-9a-f]{4}$') { return }
    }
    elseif ($Name -cmatch '^(source|tested|os\.(family|name|version|architecture)|environment|system\.chassis|memory\.[0-9]+\.(type|form)|storage\.[0-9]+\.(interface|media)|capability\.[a-z_]+(\.reason)?)$') {
        $text = [string]$Value
        if ($text -cnotmatch '^[A-Za-z0-9._+/-]+$') { return }
    }
    else {
        [Console]::Error.WriteLine("mysetup-scan: internal error: no field named $Name")
        exit 70
    }
    if ($text -ne '') { Write-Output "$Name=$text" }
}

# A class, asking only for the properties named. Nothing when the class cannot be read.
function Get-Part([string]$Class, [string[]]$Property, [string]$Namespace = 'root\cimv2') {
    try {
        return @(Get-CimInstance -Namespace $Namespace -ClassName $Class -Property $Property -ErrorAction Stop)
    }
    catch {
        return @()
    }
}

function Get-Environment {
    # HypervisorPresent is no help: it is also true on a physical computer with WSL or Hyper-V on.
    $system = @(Get-Part 'Win32_ComputerSystem' @('Manufacturer', 'Model'))
    if ($system.Count -eq 0) { return 'unknown' }
    $maker = [string]$system[0].Manufacturer
    $model = [string]$system[0].Model
    if ($model -match '^(Virtual Machine|VirtualBox|KVM|HVM domU|Bochs)' -or $model -match 'VMware|Standard PC|QEMU|Parallels' -or
        $maker -match 'VMware|innotek|QEMU|Xen|Parallels') {
        return 'virtual_machine'
    }
    return 'physical'
}

function Write-Capability([string]$Name, [string]$State) {
    if ($State -eq 'available') {
        Write-Field "capability.$Name" 'available'
    }
    else {
        Write-Field "capability.$Name" 'unavailable'
        Write-Field "capability.$Name.reason" $State
    }
}

function Get-MemoryType([object]$Code) {
    switch ([int]$Code) {
        20 { 'DDR' } 21 { 'DDR2' } 24 { 'DDR3' } 26 { 'DDR4' } 27 { 'LPDDR' } 28 { 'LPDDR2' } 29 { 'LPDDR3' }
        30 { 'LPDDR4' } 34 { 'DDR5' } 35 { 'LPDDR5' }
    }
}

function Get-MemoryForm([object]$Code) {
    switch ([int]$Code) {
        8 { 'DIMM' } 12 { 'SODIMM' } 11 { 'RIMM' } 13 { 'SRIMM' } 15 { 'FB-DIMM' }
    }
}

function Get-BusName([object]$Code) {
    switch ([int]$Code) {
        1 { 'scsi' } 3 { 'ata' } 7 { 'usb' } 8 { 'raid' } 9 { 'iscsi' } 10 { 'sas' } 11 { 'sata' } 12 { 'sd' }
        13 { 'mmc' } 17 { 'nvme' }
    }
}

function Get-InterfaceName([string]$Name) {
    switch ($Name) {
        'IDE' { 'ata' } 'SCSI' { 'scsi' } 'USB' { 'usb' } 'SATA' { 'sata' }
    }
}

$Architecture = $env:PROCESSOR_ARCHITEW6432
if (-not $Architecture) { $Architecture = $env:PROCESSOR_ARCHITECTURE }
$Architecture = switch ($Architecture) {
    'AMD64' { 'x86-64' }
    'ARM64' { 'arm64' }
    'x86' { 'x86' }
    default { 'other' }
}

# A system counts as tested once a real machine has passed; see the README.
$Tested = if ($Architecture -eq 'x86-64') { 'yes' } else { 'no' }

$Capability = @{
    cpu = 'no-source'; gpu = 'no-source'; motherboard = 'no-source'; memory_modules = 'no-source'
    configured_memory = 'no-source'; storage = 'no-source'
}

Write-Host "MySetup scan $Version. It reads hardware names and sends nothing anywhere."
Write-Host 'Copy everything from "mysetup-report 1" to "mysetup-report-end" and paste it on your build page.'
Write-Host ''

$Environment = Get-Environment

Write-Output 'mysetup-report 1'
Write-Field 'source' "script-ps1/$Version"
Write-Field 'tested' $Tested
Write-Field 'os.family' 'windows'
Write-Field 'os.name' 'windows'
# Only the version. The same class also holds the computer's name, the registered user and the serial.
$System = @(Get-Part 'Win32_OperatingSystem' @('Version'))
if ($System.Count -gt 0) { Write-Field 'os.version' $System[0].Version }
Write-Field 'os.architecture' $Architecture
Write-Field 'environment' $Environment

# The computer's own model, as its maker names it (the operator, 2026-11-13: "we need to match models"), only on a physical
# computer: a virtual machine's can be set to anything, its host's name among them. Only the properties named are asked for:
# never IdentifyingNumber (the serial number), UUID, SerialNumber or SMBIOSAssetTag.
if ($Environment -eq 'physical') {
    $Computer = @(Get-Part 'Win32_ComputerSystem' @('Manufacturer', 'Model', 'SystemFamily', 'SystemSKUNumber'))
    $Product = @(Get-Part 'Win32_ComputerSystemProduct' @('Version'))
    $Enclosure = @(Get-Part 'Win32_SystemEnclosure' @('ChassisTypes'))
    if ($Computer.Count -gt 0) {
        Write-Field 'system.vendor' $Computer[0].Manufacturer
        Write-Field 'system.model' $Computer[0].Model
        Write-Field 'system.family' $Computer[0].SystemFamily
    }
    if ($Product.Count -gt 0) { Write-Field 'system.version' $Product[0].Version }
    if ($Computer.Count -gt 0) { Write-Field 'system.sku' $Computer[0].SystemSKUNumber }
    if ($Enclosure.Count -gt 0 -and @($Enclosure[0].ChassisTypes).Count -gt 0) {
        $Chassis = [int](@($Enclosure[0].ChassisTypes)[0]) -band 0x7F
        if ($ChassisNames.ContainsKey($Chassis)) { Write-Field 'system.chassis' $ChassisNames[$Chassis] }
    }
}

# CPU. Counts add up across sockets.
$Processors = @(Get-Part 'Win32_Processor' @('Manufacturer', 'Name', 'NumberOfCores', 'NumberOfLogicalProcessors'))
if ($Processors.Count -gt 0) {
    $Maker = [string]$Processors[0].Manufacturer
    if ($Maker -eq 'GenuineIntel') { $Maker = 'Intel' }
    elseif ($Maker -eq 'AuthenticAMD') { $Maker = 'AMD' }
    Write-Field 'cpu.vendor' $Maker
    Write-Field 'cpu.model' $Processors[0].Name
    Write-Field 'cpu.cores' (($Processors | Measure-Object -Property NumberOfCores -Sum).Sum)
    Write-Field 'cpu.threads' (($Processors | Measure-Object -Property NumberOfLogicalProcessors -Sum).Sum)
    $Capability.cpu = 'available'
}

# Graphics. Only the IDs are taken from the device path; the rest of it is specific to this computer.
# Virtual adapters (remote desktop, display software) sit under ROOT\ and are not parts of the build.
$Number = 0
foreach ($Adapter in @(Get-Part 'Win32_VideoController' @('Name', 'PNPDeviceID'))) {
    $Device = [string]$Adapter.PNPDeviceID
    if ($Device -notmatch '^(PCI|ACPI)\\') { continue }
    if ($Number -ge $MaxItems) { [void]$script:Notes.Add("Only the first $MaxItems graphics adapters are listed."); break }
    if ($Device -match '^PCI\\VEN_([0-9A-Fa-f]{4})&DEV_([0-9A-Fa-f]{4})') {
        Write-Field "gpu.$Number.pci" ("{0}:{1}" -f $Matches[1].ToLowerInvariant(), $Matches[2].ToLowerInvariant())
    }
    if ($Device -match '&SUBSYS_([0-9A-Fa-f]{4})([0-9A-Fa-f]{4})') {
        Write-Field "gpu.$Number.subsystem" ("{0}:{1}" -f $Matches[2].ToLowerInvariant(), $Matches[1].ToLowerInvariant())
    }
    Write-Field "gpu.$Number.model" $Adapter.Name
    $Number++
}
if ($Number -gt 0) { $Capability.gpu = 'available' }

# Motherboard.
$Board = @(Get-Part 'Win32_BaseBoard' @('Manufacturer', 'Product'))
if ($Board.Count -gt 0) {
    $Vendor = Get-CleanText $Board[0].Manufacturer
    $Model = Get-CleanText $Board[0].Product
    Write-Field 'motherboard.vendor' $Vendor
    Write-Field 'motherboard.model' $Model
    if ($Vendor -or $Model) { $Capability.motherboard = 'available' }
}

# Memory.
$Totals = @(Get-Part 'Win32_ComputerSystem' @('TotalPhysicalMemory'))
if ($Totals.Count -gt 0 -and $Totals[0].TotalPhysicalMemory) {
    Write-Field 'memory.total_mb' ([math]::Floor([double]$Totals[0].TotalPhysicalMemory / 1048576))
}
$Number = 0
$Modules = @(Get-Part 'Win32_PhysicalMemory' @('Manufacturer', 'PartNumber', 'Capacity', 'Speed', 'ConfiguredClockSpeed', 'SMBIOSMemoryType', 'FormFactor'))
foreach ($Module in $Modules) {
    if (-not $Module.Capacity) { continue }
    if ($Number -ge $MaxItems) { [void]$script:Notes.Add("Only the first $MaxItems memory modules are listed."); break }
    Write-Field "memory.$Number.size_mb" ([math]::Floor([double]$Module.Capacity / 1048576))
    Write-Field "memory.$Number.maker" $Module.Manufacturer
    Write-Field "memory.$Number.part" $Module.PartNumber
    Write-Field "memory.$Number.speed_mts" $Module.Speed
    Write-Field "memory.$Number.configured_speed_mts" $Module.ConfiguredClockSpeed
    Write-Field "memory.$Number.type" (Get-MemoryType $Module.SMBIOSMemoryType)
    Write-Field "memory.$Number.form" (Get-MemoryForm $Module.FormFactor)
    if ($Module.ConfiguredClockSpeed) { $Capability.configured_memory = 'available' }
    $Number++
}
if ($Number -gt 0) { $Capability.memory_modules = 'available' }

# Storage: the Storage module's class, which knows NVMe and SSD, and then the older class.
$Number = 0
$Disks = @(Get-Part 'MSFT_PhysicalDisk' @('FriendlyName', 'Model', 'Size', 'BusType', 'MediaType') 'root\Microsoft\Windows\Storage')
foreach ($Disk in $Disks) {
    if (-not $Disk.Size) { continue }
    # 14 is a virtual disk, 15 a file-backed one: not parts of the build.
    if ([int]$Disk.BusType -eq 14 -or [int]$Disk.BusType -eq 15) { continue }
    if ($Number -ge $MaxItems) { [void]$script:Notes.Add("Only the first $MaxItems disks are listed."); break }
    $Name = if ($Disk.Model) { $Disk.Model } else { $Disk.FriendlyName }
    Write-Field "storage.$Number.model" $Name
    Write-Field "storage.$Number.size_bytes" ([string][uint64]$Disk.Size)
    Write-Field "storage.$Number.interface" (Get-BusName $Disk.BusType)
    switch ([int]$Disk.MediaType) {
        3 { Write-Field "storage.$Number.media" 'hdd' }
        4 { Write-Field "storage.$Number.media" 'ssd' }
    }
    $Number++
}
if ($Number -eq 0) {
    foreach ($Disk in @(Get-Part 'Win32_DiskDrive' @('Model', 'Size', 'InterfaceType'))) {
        if (-not $Disk.Size) { continue }
        if ($Number -ge $MaxItems) { [void]$script:Notes.Add("Only the first $MaxItems disks are listed."); break }
        Write-Field "storage.$Number.model" $Disk.Model
        Write-Field "storage.$Number.size_bytes" ([string][uint64]$Disk.Size)
        Write-Field "storage.$Number.interface" (Get-InterfaceName ([string]$Disk.InterfaceType))
        $Number++
    }
}
if ($Number -gt 0) { $Capability.storage = 'available' }

Write-Capability 'cpu' $Capability.cpu
Write-Capability 'gpu' $Capability.gpu
Write-Capability 'motherboard' $Capability.motherboard
Write-Capability 'memory_modules' $Capability.memory_modules
Write-Capability 'configured_memory' $Capability.configured_memory
Write-Capability 'storage' $Capability.storage
Write-Capability 'displays' 'not-collected'
Write-Output 'mysetup-report-end'

if ($Tested -eq 'no') {
    [void]$script:Notes.Add("Untested on this system (windows/$Architecture). Parts may be missing or wrong, so please check the report before you paste it.")
}
if ($Environment -eq 'virtual_machine') {
    [void]$script:Notes.Add('This looks like a virtual machine, so the report shows what it was given, not the real computer.')
}
if ($script:Notes.Count -gt 0) {
    Write-Host ''
    foreach ($Line in $script:Notes) { Write-Host $Line }
}
