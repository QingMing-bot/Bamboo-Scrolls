# 安装ipmitool (Windows版)
$downloadUrl = "https://github.com/ipmitool/ipmitool/releases/download/IPMITOOL_1_8_19/ipmitool-1.8.19-win64.zip"
$zipPath = "$env:TEMP\ipmitool.zip"
$installPath = "$env:ProgramFiles\ipmitool"

# 下载并安装
Invoke-WebRequest $downloadUrl -OutFile $zipPath
Expand-Archive -Path $zipPath -DestinationPath $installPath -Force

# 添加到系统PATH
$currentPath = [Environment]::GetEnvironmentVariable('Path', 'Machine')
if(-not $currentPath.Contains($installPath)) {
    $newPath = $currentPath + ";" + $installPath
    [Environment]::SetEnvironmentVariable('Path', $newPath, 'Machine')
    $env:Path += ";$installPath"
}

Write-Host "IPMITool安装完成！请重新启动终端"
