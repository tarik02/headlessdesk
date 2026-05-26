# Windows Service and Autostart

Windows release archives contain two executables:

- `headlessdesk.exe`: console CLI for terminals and debugging.
- `headlessdeskw.exe`: no-console build for background `serve` use.

The no-console binary is the same program built with the Windows GUI subsystem,
so it does not open a console window when Task Scheduler, Explorer, or a service
wrapper starts it.

## Native Windows Backend

The native `windows` backend captures and controls the logged-in desktop with
Win32 APIs. It must run in the interactive user session. A classic Windows
Service runs in Session 0, where desktop capture may fail with errors such as
`BitBlt: Access is denied`.

Use a logon Scheduled Task for the native backend:

```powershell
$install = "$HOME\.bin"
New-Item -ItemType Directory -Force -Path $install, "$HOME\.config\headlessdesk" | Out-Null

# Extract the release zip, then copy all extracted files into $install.
Copy-Item .\headlessdesk-windows-amd64\* $install -Force
```

Create `~\.config\headlessdesk\config.yaml`:

```yaml
server:
  listen_addr: "127.0.0.1:4243"
  mcp_path: "/mcp"
  enable_http_api: true
  enable_mcp_api: true

input: "local"
output: "local"

backends:
  local:
    type: "windows"
```

Install and start the task from an elevated PowerShell in the extracted release
directory:

```powershell
powershell -ExecutionPolicy Bypass -File .\install-task.ps1
```

The task starts `headlessdeskw.exe serve --config <config>` at user logon and
starts it immediately during installation. Check it with:

```powershell
Get-ScheduledTask -TaskName HeadlessDesk
Invoke-RestMethod http://127.0.0.1:4243/healthz
Invoke-WebRequest -Method Post http://127.0.0.1:4243/screenshot -OutFile screenshot.png
```

Uninstall it with:

```powershell
powershell -ExecutionPolicy Bypass -File .\uninstall-task.ps1
```

## Classic Service

Use a classic Windows Service only for backends that do not need the logged-in
desktop, such as RDP, VNC, or command backends targeting another session or
machine. With NSSM installed:

```powershell
nssm install HeadlessDesk "$HOME\.bin\headlessdeskw.exe" serve --config "$HOME\.config\headlessdesk\config.yaml"
nssm set HeadlessDesk AppDirectory "$HOME\.bin"
nssm set HeadlessDesk Start SERVICE_AUTO_START
nssm start HeadlessDesk
```

For the native `windows` backend, prefer the Scheduled Task above.
