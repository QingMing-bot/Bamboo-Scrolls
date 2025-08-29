Param(
    [switch]$Wails,   # use wails build (needs wails CLI)
    [switch]$Race,    # enable -race
    [switch]$Dev,     # build with dev tag instead of production
    [switch]$Clean    # clean output
)

$ErrorActionPreference = 'Stop'

$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $Root

$Bin = Join-Path $Root 'bin'
$Dist = Join-Path $Root 'dist'

if($Clean){
    if(Test-Path $Bin){ Remove-Item $Bin -Recurse -Force }
    if(Test-Path $Dist){ Remove-Item $Dist -Recurse -Force }
    Write-Host 'Cleaned bin/ dist/' -ForegroundColor Cyan
    exit 0
}

if(-not (Get-Command go -ErrorAction SilentlyContinue)) { throw 'go not found' }

go mod tidy

New-Item -ItemType Directory -Force -Path $Bin | Out-Null

if($Wails){
    if(-not (Get-Command wails -ErrorAction SilentlyContinue)) { throw 'wails CLI missing: npm install -g @wails/cli' }
    Write-Host '=> Wails build' -ForegroundColor Green
    wails build
    Write-Host 'Artifacts: dist/' -ForegroundColor Yellow
    exit 0
}

$ldFlags = '-s -w'
$raceFlag = ''
if($Race){ $raceFlag = '-race' }

Write-Host '=> Go build' -ForegroundColor Green
$tag = 'production'
if($Dev){ $tag = 'dev' }
go build $raceFlag -tags $tag -ldflags $ldFlags -o (Join-Path $Bin 'ipmi-ssh-manager.exe') ./cmd/app

Write-Host "Done -> bin/ipmi-ssh-manager.exe (tag=$tag)" -ForegroundColor Cyan
