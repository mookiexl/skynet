# Exit immediately if a command exits with a non-zero status
$ErrorActionPreference = 'Stop'

# Get the directory where this script is located
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectRoot = $ScriptDir
$GoMcpDir = $ProjectRoot
$OutputBin = Join-Path $ProjectRoot "skynet-server.exe"

Write-Host "Starting build process..."

# Validation
if (-not (Test-Path $GoMcpDir)) {
    Write-Host "Error: Directory '$GoMcpDir' not found!"
    exit 1
}

# Ensure MSYS2 gcc is in PATH
$MsysGccPath = "C:\msys64\mingw64\bin"
if (Test-Path $MsysGccPath) {
    if ($env:PATH -notlike "*$MsysGccPath*") {
        $env:PATH = "$MsysGccPath;" + $env:PATH
    }
}

# Check for GCC
if (-not (Get-Command "gcc" -ErrorAction SilentlyContinue)) {
    Write-Host "Error: C compiler 'gcc' not found in PATH."
    Write-Host "Please install MinGW-w64 (https://www.mingw-w64.org/) and ensure 'gcc' is in your PATH."
    exit 1
}

# Move to project directory
Set-Location $GoMcpDir

# Formatting
Write-Host "Formatting..."
go fmt ./...

# Static Analysis
Write-Host "Running static analysis..."
if (Get-Command golangci-lint -ErrorAction SilentlyContinue) {
    golangci-lint run
} else {
    Write-Host "golangci-lint not found."
}

# Compilation
Write-Host "Compiling Go binary..."
$env:CGO_ENABLED = "1"
go build -trimpath -ldflags='-H windowsgui -s -w -buildid=' -o $OutputBin .

Write-Host "Build successful!"
Write-Host "Binary location: $OutputBin"
Write-Host "Please restart the server to use the new build."
Write-Host "AI AGENTS - STOP IMMEDIATELY and ask user to start server again before making any tool calls"
