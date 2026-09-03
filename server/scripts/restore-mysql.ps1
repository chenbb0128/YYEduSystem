param(
  [Parameter(Mandatory = $true)]
  [string]$BackupFile,
  [string]$HostName = $(if ($env:TUOGUAN_BACKUP_MYSQL_HOST) { $env:TUOGUAN_BACKUP_MYSQL_HOST } else { "127.0.0.1" }),
  [int]$Port = $(if ($env:TUOGUAN_BACKUP_MYSQL_PORT) { [int]$env:TUOGUAN_BACKUP_MYSQL_PORT } else { 3306 }),
  [string]$UserName = $(if ($env:TUOGUAN_BACKUP_MYSQL_USER) { $env:TUOGUAN_BACKUP_MYSQL_USER } else { "tuoguan_system" }),
  [string]$DatabaseName = $(if ($env:TUOGUAN_BACKUP_MYSQL_DATABASE) { $env:TUOGUAN_BACKUP_MYSQL_DATABASE } else { "tuoguan_system" })
)

$ErrorActionPreference = "Stop"
$resolvedBackup = [System.IO.Path]::GetFullPath($BackupFile)
if (-not (Test-Path -LiteralPath $resolvedBackup -PathType Leaf)) { throw "备份文件不存在：$resolvedBackup" }
if (-not $env:TUOGUAN_BACKUP_MYSQL_PASSWORD) { throw "请通过 TUOGUAN_BACKUP_MYSQL_PASSWORD 提供恢复账号密码。" }
$confirmation = Read-Host "这会覆盖数据库 $DatabaseName。请输入 RESTORE 继续"
if ($confirmation -ne "RESTORE") { throw "已取消恢复" }

$mysqlArgs = @("--host=$HostName", "--port=$Port", "--user=$UserName", $DatabaseName)
try {
  $env:MYSQL_PWD = $env:TUOGUAN_BACKUP_MYSQL_PASSWORD
  Get-Content -Raw -LiteralPath $resolvedBackup | & mysql @mysqlArgs
  if ($LASTEXITCODE -ne 0) { throw "mysql 恢复失败，退出码 $LASTEXITCODE" }
} finally {
  Remove-Item Env:MYSQL_PWD -ErrorAction SilentlyContinue
}
Write-Output "恢复完成：$resolvedBackup"
