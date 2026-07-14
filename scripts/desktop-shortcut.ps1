param([ValidateSet('install','remove')][string]$Action = 'install')
$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$desktop = [Environment]::GetFolderPath('Desktop')
if (-not $desktop) { throw 'Desktop folder could not be resolved for this user.' }
$shortcut = Join-Path $desktop 'DataGuardian.lnk'
if ($Action -eq 'remove') {
  Remove-Item -LiteralPath $shortcut -Force -ErrorAction SilentlyContinue
  Write-Host "DataGuardian shortcut removed: $shortcut"
  exit 0
}
$launcher = Join-Path $root 'start-dataguardian.bat'
if (-not (Test-Path -LiteralPath $launcher -PathType Leaf)) { throw "Launcher not found: $launcher" }
$shell = New-Object -ComObject WScript.Shell
$link = $shell.CreateShortcut($shortcut)
$link.TargetPath = $launcher
$link.WorkingDirectory = $root
$link.Description = 'Start DataGuardian with Docker Compose'
$link.Save()
Write-Host "DataGuardian shortcut installed: $shortcut"
