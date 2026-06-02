[CmdletBinding()]
param(
    [string]$OutputRoot = "desktop_app\ffmpeg\android",
    [string]$BaseUrl = "https://sourceforge.net/projects/xabe-ffmpeg.mirror/files/executables"
)

$ErrorActionPreference = "Stop"

$items = @(
    @{ Abi = "armeabi-v7a"; Zip = "ffmpeg-android-arm.zip" },
    @{ Abi = "arm64-v8a";  Zip = "ffmpeg-android-arm64.zip" },
    @{ Abi = "x86";         Zip = "ffmpeg-android-x86.zip" },
    @{ Abi = "x86_64";      Zip = "ffmpeg-android-x86_64.zip" }
)

$outputRootFull = [IO.Path]::GetFullPath($OutputRoot)
New-Item -ItemType Directory -Path $outputRootFull -Force | Out-Null

$workRoot = Join-Path ([IO.Path]::GetTempPath()) ("music-dl-android-ffmpeg-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $workRoot -Force | Out-Null

try {
    foreach ($item in $items) {
        $zipPath = Join-Path $workRoot $item.Zip
        $extractRoot = Join-Path $workRoot $item.Abi
        $abiRoot = Join-Path $outputRootFull $item.Abi
        $url = "$BaseUrl/$($item.Zip)/download"

        Write-Host "Downloading $($item.Zip)"
        & curl.exe -L --fail --retry 5 --retry-delay 5 -o $zipPath $url
        if ($LASTEXITCODE -ne 0) {
            throw "curl failed for $url"
        }

        Expand-Archive -Path $zipPath -DestinationPath $extractRoot -Force
        New-Item -ItemType Directory -Path $abiRoot -Force | Out-Null

        foreach ($tool in @("ffmpeg", "ffprobe")) {
            $source = Join-Path $extractRoot $tool
            if (-not (Test-Path $source)) {
                throw "$($item.Zip) does not contain $tool"
            }

            $target = Join-Path $abiRoot $tool
            Copy-Item -LiteralPath $source -Destination $target -Force

            $length = (Get-Item -LiteralPath $target).Length
            if ($length -lt 1048576) {
                throw "$target is unexpectedly small ($length bytes)"
            }
        }
    }
}
finally {
    Remove-Item -LiteralPath $workRoot -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host "Android ffmpeg binaries prepared at $outputRootFull"
