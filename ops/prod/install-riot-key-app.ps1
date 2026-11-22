#Requires -Version 5.1
[CmdletBinding()]
param([string]$Python = 'python')

$ErrorActionPreference = 'Stop'
$appName = 'WinRift Key Updater'
$pythonExe = & $Python -B -c 'import sys, tkinter; print(sys.executable)'
if ($LASTEXITCODE -ne 0 -or -not $pythonExe) {
    throw 'Install Python 3 with Tkinter or pass its full path with -Python.'
}
$pythonw = Join-Path (Split-Path -Parent $pythonExe) 'pythonw.exe'
if (-not (Test-Path -LiteralPath $pythonw -PathType Leaf)) {
    throw 'The selected Python installation needs pythonw.exe.'
}

$desktop = [Environment]::GetFolderPath('DesktopDirectory')
$startMenu = [Environment]::GetFolderPath('Programs')
foreach ($folder in @($desktop, $startMenu)) {
    if (-not $folder -or -not (Test-Path -LiteralPath $folder -PathType Container)) {
        throw 'Windows could not locate your Desktop or Start menu folder.'
    }
}

$programs = Join-Path ([Environment]::GetFolderPath('LocalApplicationData')) 'Programs'
New-Item -ItemType Directory -Path $programs -Force | Out-Null
$programs = (Resolve-Path -LiteralPath $programs).ProviderPath
$install = Join-Path $programs $appName
$entry = Join-Path $install 'riot-key-app.py'
if ((Test-Path -LiteralPath $install) -and -not (Test-Path -LiteralPath $entry -PathType Leaf)) {
    throw "Another item is already using $install."
}
$repo = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '../..')).ProviderPath
$brand = Join-Path $repo 'apps/web/public/images/brand/winrift-icon.png'
$work = Join-Path $programs ('winrift-updater-install-' + [guid]::NewGuid().ToString('N'))
$staged = Join-Path $work 'app'
$backup = Join-Path $work 'previous'
New-Item -ItemType Directory -Path $staged | Out-Null

try {
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'riot-key-app.py'),(Join-Path $PSScriptRoot 'riot_key_client.py'),$brand -Destination $staged

    Add-Type -AssemblyName System.Drawing
    $source = [Drawing.Image]::FromFile($brand)
    $bitmap = [Drawing.Bitmap]::new(256, 256)
    $graphics = [Drawing.Graphics]::FromImage($bitmap)
    $png = [IO.MemoryStream]::new()
    try {
        $graphics.InterpolationMode = [Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
        $scale = 256 / [Math]::Max($source.Width, $source.Height)
        $width = [int]($source.Width * $scale)
        $height = [int]($source.Height * $scale)
        $graphics.DrawImage($source, [int]((256 - $width) / 2), [int]((256 - $height) / 2), $width, $height)
        $bitmap.Save($png, [Drawing.Imaging.ImageFormat]::Png)
        $writer = [IO.BinaryWriter]::new([IO.File]::Create((Join-Path $staged 'winrift.ico')))
        try {
            # One 256-pixel PNG in an ICO directory entry. Zero encodes 256 in the width and height fields.
            $writer.Write([byte[]]@(0, 0, 1, 0, 1, 0, 0, 0, 0, 0, 1, 0, 32, 0))
            $writer.Write([uint32]$png.Length)
            $writer.Write([uint32]22)
            $writer.Write($png.ToArray())
        } finally {
            $writer.Dispose()
        }
    } finally {
        $png.Dispose()
        $graphics.Dispose()
        $bitmap.Dispose()
        $source.Dispose()
    }

    if ((Resolve-Path -LiteralPath $staged).ProviderPath -ne $staged -or
        [IO.Path]::GetDirectoryName($install) -ne $programs -or
        [IO.Path]::GetDirectoryName($backup) -ne $work) {
        throw 'The app staging folder is outside the expected location.'
    }
    if (Test-Path -LiteralPath $install) {
        $existing = Get-Item -LiteralPath $install
        if ($existing.FullName -ne $install -or ($existing.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
            throw 'The existing app folder is outside the expected location.'
        }
        [IO.Directory]::Move($install, $backup)
    }
    try {
        [IO.Directory]::Move($staged, $install)
    } catch {
        if (Test-Path -LiteralPath $backup) { [IO.Directory]::Move($backup, $install) }
        throw
    }

    $shell = New-Object -ComObject WScript.Shell
    foreach ($folder in @($desktop, $startMenu)) {
        $linkPath = Join-Path $folder "$appName.lnk"
        $link = $shell.CreateShortcut($linkPath)
        $link.TargetPath = $pythonw
        $link.Arguments = '-B "' + $entry + '"'
        $link.WorkingDirectory = $install
        $link.IconLocation = "$(Join-Path $install 'winrift.ico'),0"
        $link.Description = 'Update the WinRift Riot API key over SSH'
        $link.Save()
        Write-Output $linkPath
    }
} finally {
    $resolvedWork = (Resolve-Path -LiteralPath $work).ProviderPath
    $needsRecovery = (Test-Path -LiteralPath $backup) -and -not (Test-Path -LiteralPath $install)
    if ($resolvedWork -eq $work -and [IO.Path]::GetDirectoryName($resolvedWork) -eq $programs -and
        -not $needsRecovery) {
        Remove-Item -LiteralPath $resolvedWork -Recurse -Force
    }
}
