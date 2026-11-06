<#
    Tests scripts/mysetup-scan.ps1 on a made-up computer, by shadowing Get-CimInstance. Every class also
    answers with planted values the report must never carry, and the real computer is never asked.

        powershell -NoProfile -ExecutionPolicy Bypass -File tests\test_scan_ps1.ps1 [-ReportPath report.txt]

    -ReportPath saves the main report for tests/validate_report.py.
#>
param([string]$ReportPath)

$ErrorActionPreference = 'Stop'
$script:Failures = 0
$Script = Join-Path (Split-Path -Parent $PSScriptRoot) 'scripts\mysetup-scan.ps1'

function Assert-That([bool]$Condition, [string]$What) {
    if ($Condition) { Write-Output "ok   $What" }
    else { Write-Output "FAIL $What"; $script:Failures++ }
}

# What must never come through, on any made-up computer.
$Planted = @('FAKE-SERIAL', 'FAKE-HOST', 'FAKE-USER', 'FAKE-DOMAIN', 'FAKE-PRODUCT-ID', 'FAKE-TAG', 'FAKE-LOCATOR',
    'FAKE-UNIQUE', 'FAKE-INSTANCE', 'de:ad:be:ef:00:01', '10.20.30.40', '12345678-1234-1234-1234-123456789abc')

function New-Fake([hashtable]$Values) { return [pscustomobject]$Values }

$global:Computer = @{
    Win32_ComputerSystem  = @(New-Fake @{ Manufacturer = 'ASUS'; Model = 'System Product Name'; TotalPhysicalMemory = [uint64]34359738368
            Name = 'FAKE-HOST'; UserName = 'FAKE-USER'; Domain = 'FAKE-DOMAIN'; HypervisorPresent = $true })
    Win32_OperatingSystem = @(New-Fake @{ Version = '10.0.26100'; CSName = 'FAKE-HOST'; RegisteredUser = 'FAKE-USER'; SerialNumber = 'FAKE-PRODUCT-ID'
            Caption = 'Microsoft Windows 11 Pro' })
    Win32_Processor       = @(New-Fake @{ Manufacturer = 'GenuineIntel'; Name = 'Intel(R) Core(TM) i7-14700K CPU @ 3.40GHz'; NumberOfCores = 20
            NumberOfLogicalProcessors = 28; ProcessorId = 'FAKE-SERIAL'; SerialNumber = 'FAKE-SERIAL'; SystemName = 'FAKE-HOST' })
    Win32_VideoController = @(
        New-Fake @{ Name = 'NVIDIA GeForce RTX 4070'; PNPDeviceID = 'PCI\VEN_10DE&DEV_2786&SUBSYS_89431043&REV_A1\4&FAKE-INSTANCE&0&0019'; SystemName = 'FAKE-HOST' }
        New-Fake @{ Name = 'Parsec Virtual Display Adapter'; PNPDeviceID = 'ROOT\DISPLAY\0000'; SystemName = 'FAKE-HOST' }
    )
    Win32_BaseBoard       = @(New-Fake @{ Manufacturer = 'ASUSTeK COMPUTER INC.'; Product = 'ROG STRIX Z790-E'; SerialNumber = 'FAKE-SERIAL'; Tag = 'FAKE-TAG' })
    Win32_PhysicalMemory  = @(
        New-Fake @{ Manufacturer = 'Corsair'; PartNumber = 'CMK32GX5M2B6000C30   '; Capacity = [uint64]17179869184; Speed = 6000; ConfiguredClockSpeed = 5600
            SMBIOSMemoryType = 34; FormFactor = 8; SerialNumber = 'FAKE-SERIAL'; Tag = 'FAKE-TAG'; DeviceLocator = 'FAKE-LOCATOR'; BankLabel = 'FAKE-LOCATOR' }
        New-Fake @{ Manufacturer = 'Corsair'; PartNumber = 'CMK32GX5M2B6000C30'; Capacity = [uint64]17179869184; Speed = 6000; ConfiguredClockSpeed = 5600
            SMBIOSMemoryType = 34; FormFactor = 8; SerialNumber = 'FAKE-SERIAL'; Tag = 'FAKE-TAG'; DeviceLocator = 'FAKE-LOCATOR'; BankLabel = 'FAKE-LOCATOR' }
    )
    MSFT_PhysicalDisk     = @(
        New-Fake @{ FriendlyName = 'Samsung SSD 990 PRO 2TB'; Model = 'Samsung SSD 990 PRO 2TB'; Size = [uint64]2000398934016; BusType = 17; MediaType = 4
            SerialNumber = 'FAKE-SERIAL'; UniqueId = 'FAKE-UNIQUE'; ObjectId = 'FAKE-UNIQUE' }
        New-Fake @{ FriendlyName = 'Msft Virtual Disk'; Model = 'Virtual Disk'; Size = [uint64]1099511627776; BusType = 14; MediaType = 0
            SerialNumber = 'FAKE-SERIAL'; UniqueId = 'FAKE-UNIQUE'; ObjectId = 'FAKE-UNIQUE' }
        New-Fake @{ FriendlyName = 'WDC WD40EFRX'; Model = 'WDC WD40EFRX'; Size = [uint64]4000787030016; BusType = 11; MediaType = 3
            SerialNumber = 'FAKE-SERIAL'; UniqueId = 'FAKE-UNIQUE'; ObjectId = 'FAKE-UNIQUE' }
    )
    Win32_DiskDrive       = @(New-Fake @{ Model = 'Old Disk'; Size = [uint64]500107862016; InterfaceType = 'IDE'; SerialNumber = 'FAKE-SERIAL'; PNPDeviceID = 'FAKE-INSTANCE' })
}

# Shadows the cmdlet for the script run below, and answers only what the query names, as the cmdlet does.
$global:Asked = New-Object System.Collections.ArrayList
function Get-CimInstance {
    [CmdletBinding()]
    param([string]$Namespace, [string]$ClassName, [string[]]$Property)
    [void]$global:Asked.Add(@{ Class = $ClassName; Property = $Property })
    if (-not $global:Computer.ContainsKey($ClassName)) { throw "no class $ClassName" }
    return $global:Computer[$ClassName] | Select-Object -Property *
}

function Invoke-Scan {
    param([hashtable]$Environment = @{})
    $saved = @{}
    foreach ($key in $Environment.Keys) {
        $saved[$key] = [Environment]::GetEnvironmentVariable($key)
        [Environment]::SetEnvironmentVariable($key, $Environment[$key])
    }
    $lines = & $Script 6>$null
    foreach ($key in $saved.Keys) { [Environment]::SetEnvironmentVariable($key, $saved[$key]) }
    return @($lines)
}

function Get-ReportText([object[]]$Lines) { return ($Lines -join "`n") }

$Original = Invoke-Scan -Environment @{ PROCESSOR_ARCHITECTURE = 'AMD64'; PROCESSOR_ARCHITEW6432 = $null }
$Text = Get-ReportText $Original

$Expected = @'
mysetup-report 1
source=script-ps1/0.1.0
tested=yes
os.family=windows
os.name=windows
os.version=10.0.26100
os.architecture=x86-64
environment=physical
cpu.vendor=Intel
cpu.model=Intel(R) Core(TM) i7-14700K CPU @ 3.40GHz
cpu.cores=20
cpu.threads=28
gpu.0.pci=10de:2786
gpu.0.subsystem=1043:8943
gpu.0.model=NVIDIA GeForce RTX 4070
motherboard.vendor=ASUSTeK COMPUTER INC.
motherboard.model=ROG STRIX Z790-E
memory.total_mb=32768
memory.0.size_mb=16384
memory.0.maker=Corsair
memory.0.part=CMK32GX5M2B6000C30
memory.0.speed_mts=6000
memory.0.configured_speed_mts=5600
memory.0.type=DDR5
memory.0.form=DIMM
memory.1.size_mb=16384
memory.1.maker=Corsair
memory.1.part=CMK32GX5M2B6000C30
memory.1.speed_mts=6000
memory.1.configured_speed_mts=5600
memory.1.type=DDR5
memory.1.form=DIMM
storage.0.model=Samsung SSD 990 PRO 2TB
storage.0.size_bytes=2000398934016
storage.0.interface=nvme
storage.0.media=ssd
storage.1.model=WDC WD40EFRX
storage.1.size_bytes=4000787030016
storage.1.interface=sata
storage.1.media=hdd
capability.cpu=available
capability.gpu=available
capability.motherboard=available
capability.memory_modules=available
capability.configured_memory=available
capability.storage=available
capability.displays=unavailable
capability.displays.reason=not-collected
mysetup-report-end
'@ -replace "`r`n", "`n"

Assert-That ($Text -ceq $Expected.TrimEnd()) 'the report on a made-up computer is exactly the expected one'
if ($Text -cne $Expected.TrimEnd()) {
    Compare-Object ($Expected -split "`n") ($Text -split "`n") | ForEach-Object { Write-Output ("     " + $_.SideIndicator + " " + $_.InputObject) }
}

foreach ($Value in $Planted) {
    Assert-That (-not $Text.Contains($Value)) "the report does not carry $Value"
}

$Banned = 'SerialNumber', 'Name$', 'UserName', 'Domain', 'CSName', 'RegisteredUser', 'UniqueId', 'ObjectId', 'Tag', 'DeviceLocator', 'BankLabel', 'ProcessorId', 'SystemName', 'Caption'
$AskedFor = @($global:Asked | ForEach-Object { $_.Property } | Where-Object { $_ -ne 'Name' -and $_ -ne 'FriendlyName' })
foreach ($Name in $Banned) {
    Assert-That (-not ($AskedFor | Where-Object { $_ -match "^$Name" })) "no query asks for $Name"
}
Assert-That (@($global:Asked | Where-Object { -not $_.Property }).Count -eq 0) 'every query names its properties'

# The hypervisor flag alone does not make a virtual machine; a virtual machine's model does.
$Saved = $global:Computer.Win32_ComputerSystem
$global:Computer.Win32_ComputerSystem = @(New-Fake @{ Manufacturer = 'Microsoft Corporation'; Model = 'Virtual Machine'; TotalPhysicalMemory = [uint64]8589934592 })
$Lines = Invoke-Scan -Environment @{ PROCESSOR_ARCHITECTURE = 'AMD64'; PROCESSOR_ARCHITEW6432 = $null }
Assert-That ($Lines -contains 'environment=virtual_machine') 'a virtual machine is reported as one'
$global:Computer.Win32_ComputerSystem = $Saved

# ARM64 has not passed on a real machine.
$Lines = Invoke-Scan -Environment @{ PROCESSOR_ARCHITECTURE = 'ARM64'; PROCESSOR_ARCHITEW6432 = $null }
Assert-That (($Lines -contains 'tested=no') -and ($Lines -contains 'os.architecture=arm64')) 'Windows on ARM says it is untested'
# 32-bit PowerShell on 64-bit Windows still reports the computer's architecture.
$Lines = Invoke-Scan -Environment @{ PROCESSOR_ARCHITECTURE = 'x86'; PROCESSOR_ARCHITEW6432 = 'AMD64' }
Assert-That ($Lines -contains 'os.architecture=x86-64') '32-bit PowerShell reports the real architecture'

# The older disk class is the fallback; placeholders, escape codes and shaped values are dropped.
$SavedDisks = $global:Computer.MSFT_PhysicalDisk
$global:Computer.Remove('MSFT_PhysicalDisk')
$SavedBoard = $global:Computer.Win32_BaseBoard
$global:Computer.Win32_BaseBoard = @(New-Fake @{ Manufacturer = 'To Be Filled By O.E.M.'; Product = 'de:ad:be:ef:00:01' })
$SavedMemory = $global:Computer.Win32_PhysicalMemory
$global:Computer.Win32_PhysicalMemory = @(New-Fake @{ Manufacturer = "Bad$([char]27)[31mMaker"; PartNumber = 'Unknown'; Capacity = [uint64]8589934592; Speed = 0; ConfiguredClockSpeed = 0; SMBIOSMemoryType = 0; FormFactor = 0 })
$SavedCpu = $global:Computer.Win32_Processor
$global:Computer.Win32_Processor = @(New-Fake @{ Manufacturer = 'AuthenticAMD'; Name = 'AMD Ryzen 9 $(Remove-Item x) ; rm -rf /'; NumberOfCores = 16; NumberOfLogicalProcessors = 32 })
$Lines = Invoke-Scan -Environment @{ PROCESSOR_ARCHITECTURE = 'AMD64'; PROCESSOR_ARCHITEW6432 = $null }
$Text2 = Get-ReportText $Lines
Assert-That (($Lines -contains 'storage.0.model=Old Disk') -and ($Lines -contains 'storage.0.interface=ata')) 'the older disk class is used when the newer one cannot be read'
Assert-That (($Lines -contains 'capability.motherboard=unavailable') -and -not $Text2.Contains('motherboard.model') -and -not $Text2.Contains('motherboard.vendor')) 'a placeholder and an address-shaped board name are dropped'
Assert-That (($Lines -contains 'memory.0.maker=Bad[31mMaker') -and -not $Text2.Contains([string][char]27)) 'control characters are removed'
Assert-That (-not $Text2.Contains('memory.0.part=')) 'a placeholder part number is dropped'
Assert-That ($Lines -contains 'cpu.model=AMD Ryzen 9 $(Remove-Item x) ; rm -rf /') 'a hostile name is printed as text and not run'
Assert-That ($Lines -contains 'cpu.vendor=AMD') 'the CPU vendor is named'
$global:Computer.MSFT_PhysicalDisk = $SavedDisks
$global:Computer.Win32_BaseBoard = $SavedBoard
$global:Computer.Win32_PhysicalMemory = $SavedMemory
$global:Computer.Win32_Processor = $SavedCpu

# Nothing readable: still a whole report, saying so, and never a guess.
$SavedAll = $global:Computer
$global:Computer = @{}
$Lines = Invoke-Scan -Environment @{ PROCESSOR_ARCHITECTURE = 'AMD64'; PROCESSOR_ARCHITEW6432 = $null }
Assert-That (($Lines[0] -ceq 'mysetup-report 1') -and ($Lines[-1] -ceq 'mysetup-report-end')) 'a computer that answers nothing still gets a whole report'
Assert-That (($Lines -contains 'capability.cpu=unavailable') -and ($Lines -contains 'capability.cpu.reason=no-source') -and ($Lines -contains 'environment=unknown')) 'it says what it could not read'
$global:Computer = $SavedAll

# What the script itself must never contain.
$Source = Get-Content -Raw -LiteralPath $Script
foreach ($Word in 'Invoke-WebRequest', 'Invoke-RestMethod', 'Net.WebClient', 'HttpClient', 'Start-BitsTransfer', 'Invoke-Expression', 'Set-Content', 'Out-File', 'Add-Content', 'New-Item', 'Set-ExecutionPolicy') {
    Assert-That (-not $Source.Contains($Word)) "the script does not use $Word"
}

if ($ReportPath) { [IO.File]::WriteAllText($ReportPath, $Text + "`n") }

if ($script:Failures -gt 0) {
    Write-Output "$($script:Failures) failed"
    exit 1
}
Write-Output 'all passed'
exit 0
