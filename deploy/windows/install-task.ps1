param(
    [string]$InstallDir = (Join-Path $HOME ".bin"),
    [string]$ConfigPath = (Join-Path $HOME ".config\headlessdesk\config.yaml"),
    [string]$TaskName = "HeadlessDesk"
)

$ErrorActionPreference = "Stop"

$serverExe = Join-Path $InstallDir "headlessdeskw.exe"
if (-not (Test-Path -LiteralPath $serverExe)) {
    $serverExe = Join-Path $InstallDir "headlessdesk.exe"
}
if (-not (Test-Path -LiteralPath $serverExe)) {
    throw "headlessdesk executable not found in $InstallDir"
}
if (-not (Test-Path -LiteralPath $ConfigPath)) {
    throw "config file not found: $ConfigPath"
}

$action = New-ScheduledTaskAction `
    -Execute $serverExe `
    -Argument "serve --config `"$ConfigPath`"" `
    -WorkingDirectory $InstallDir
$trigger = New-ScheduledTaskTrigger -AtLogOn -User "$env:USERDOMAIN\$env:USERNAME"
$principal = New-ScheduledTaskPrincipal `
    -UserId "$env:USERDOMAIN\$env:USERNAME" `
    -LogonType Interactive `
    -RunLevel Highest
$settings = New-ScheduledTaskSettingsSet `
    -AllowStartIfOnBatteries `
    -DontStopIfGoingOnBatteries `
    -ExecutionTimeLimit 0 `
    -MultipleInstances IgnoreNew

Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue
Register-ScheduledTask `
    -TaskName $TaskName `
    -Action $action `
    -Trigger $trigger `
    -Principal $principal `
    -Settings $settings `
    -Description "HeadlessDesk local desktop HTTP and MCP bridge" | Out-Null
Start-ScheduledTask -TaskName $TaskName

Get-ScheduledTask -TaskName $TaskName
