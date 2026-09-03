param(
  [string]$OutputDirectory = ".\backups",
  [string]$HostName = $(if ($env:TUOGUAN_BACKUP_MYSQL_HOST) { $env:TUOGUAN_BACKUP_MYSQL_HOST } else { "127.0.0.1" }),
  [int]$Port = $(if ($env:TUOGUAN_BACKUP_MYSQL_PORT) { [int]$env:TUOGUAN_BACKUP_MYSQL_PORT } else { 3306 }),
  [string]$UserName = $(if ($env:TUOGUAN_BACKUP_MYSQL_USER) { $env:TUOGUAN_BACKUP_MYSQL_USER } else { "tuoguan_system" }),
  [string]$DatabaseName = $(if ($env:TUOGUAN_BACKUP_MYSQL_DATABASE) { $env:TUOGUAN_BACKUP_MYSQL_DATABASE } else { "tuoguan_system" })
)

$ErrorActionPreference = "Stop"
if (-not $env:TUOGUAN_BACKUP_MYSQL_PASSWORD) {
  throw "请通过 TUOGUAN_BACKUP_MYSQL_PASSWORD 提供备份账号密码，不要把密码写进命令行或脚本。"
}

$resolvedOutput = [System.IO.Path]::GetFullPath($OutputDirectory)
New-Item -ItemType Directory -Force -Path $resolvedOutput | Out-Null
$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$target = Join-Path $resolvedOutput "$DatabaseName-$stamp.sql"
$dumpArgs = @(
  "--single-transaction", "--quick", "--routines", "--events", "--triggers",
  "--set-gtid-purged=OFF", "--hex-blob", "--default-character-set=utf8mb4",
  "--host=$HostName", "--port=$Port", "--user=$UserName", $DatabaseName
)

try {
  # MYSQL_PWD is inherited only by the mysqldump child process and keeps the
  # password out of the command line and the generated backup file.
  $env:MYSQL_PWD = $env:TUOGUAN_BACKUP_MYSQL_PASSWORD
  & mysqldump @dumpArgs --result-file=$target
  if ($LASTEXITCODE -ne 0) {
    Remove-Item -LiteralPath $target -Force -ErrorAction SilentlyContinue
    throw "mysqldump 失败，退出码 $LASTEXITCODE"
  }
} finally {
  Remove-Item Env:MYSQL_PWD -ErrorAction SilentlyContinue
}

$hash = Get-FileHash -Algorithm SHA256 -LiteralPath $target
"$($hash.Hash)  $([System.IO.Path]::GetFileName($target))" | Set-Content -Encoding ascii -LiteralPath "$target.sha256"
Write-Output "备份完成：$target"
Write-Output "校验文件：$target.sha256"
