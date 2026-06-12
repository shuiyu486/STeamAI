param(
  [string]$Target = ''
)

$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$PackRoot = Split-Path -Parent $ScriptDir

function Assert-TextFile {
  param(
    [string]$Path,
    [int]$LimitBytes
  )
  if (-not (Test-Path -LiteralPath $Path)) { throw "missing file: $Path" }
  $text = [System.IO.File]::ReadAllText($Path, [System.Text.Encoding]::UTF8)
  if ([string]::IsNullOrWhiteSpace($text)) { throw "empty file: $Path" }
  $size = (Get-Item -LiteralPath $Path).Length
  if ($size -gt $LimitBytes) { throw "file too large: $Path $size > $LimitBytes" }
  [pscustomobject]@{File=$Path; Bytes=$size; Limit=$LimitBytes}
}

$rows = @()
if ([string]::IsNullOrWhiteSpace($Target)) {
  $rows += Assert-TextFile -Path (Join-Path $PackRoot 'manifest.yml') -LimitBytes 8192
  $rows += Assert-TextFile -Path (Join-Path $PackRoot 'CLAUDE.local.snippet.md') -LimitBytes 8192
  $rows += Assert-TextFile -Path (Join-Path $PackRoot 'references\vmp-re\README.md') -LimitBytes 16384
  $rows += Assert-TextFile -Path (Join-Path $PackRoot 'references\vmp-re\workflow-template.md') -LimitBytes 16384
  $rows += Assert-TextFile -Path (Join-Path $PackRoot 'references\vmp-re\progressive-disclosure.md') -LimitBytes 16384
  $rows += Assert-TextFile -Path (Join-Path $PackRoot 'references\vmp-re\toolchain-router.md') -LimitBytes 16384
  $rows += Assert-TextFile -Path (Join-Path $PackRoot 'references\vmp-re\singleton-handler-review.md') -LimitBytes 16384
  $rows += Assert-TextFile -Path (Join-Path $PackRoot 'references\vmp-re\task-handoff.template.md') -LimitBytes 12288
  $rows += Assert-TextFile -Path (Join-Path $PackRoot 'examples\tools.example.yml') -LimitBytes 8192
  $rows += Assert-TextFile -Path (Join-Path $PackRoot 'examples\project-layout.example.md') -LimitBytes 8192
  $rows += Assert-TextFile -Path (Join-Path $PackRoot 'examples\gitignore.example') -LimitBytes 8192
} else {
  $Target = [System.IO.Path]::GetFullPath($Target)
  $rows += Assert-TextFile -Path (Join-Path $Target 'CLAUDE.local.md') -LimitBytes 8192
  $claude = [System.IO.File]::ReadAllText((Join-Path $Target 'CLAUDE.local.md'), [System.Text.Encoding]::UTF8)
  if ($claude -notmatch '<!-- BEGIN vmp-re-template:router') { throw "missing managed router block in CLAUDE.local.md" }
  if ($claude -notmatch '<!-- END vmp-re-template:router -->') { throw "unterminated managed router block in CLAUDE.local.md" }

  $rows += Assert-TextFile -Path (Join-Path $Target 'references\vmp-re\README.md') -LimitBytes 16384
  $rows += Assert-TextFile -Path (Join-Path $Target 'references\vmp-re\workflow-template.md') -LimitBytes 16384
  $rows += Assert-TextFile -Path (Join-Path $Target 'references\vmp-re\progressive-disclosure.md') -LimitBytes 16384
  $rows += Assert-TextFile -Path (Join-Path $Target 'references\vmp-re\toolchain-router.md') -LimitBytes 16384
  $rows += Assert-TextFile -Path (Join-Path $Target 'references\vmp-re\singleton-handler-review.md') -LimitBytes 16384
  $rows += Assert-TextFile -Path (Join-Path $Target 'references\vmp-re\task-handoff.md') -LimitBytes 12288
  $rows += Assert-TextFile -Path (Join-Path $Target '.re-template.yml') -LimitBytes 8192
}

$rows | ForEach-Object {
  $display = $_.File
  if (-not [string]::IsNullOrWhiteSpace($Target) -and $_.File.StartsWith($Target)) {
    $display = $_.File.Substring($Target.Length).TrimStart('\')
  } elseif ($_.File.StartsWith($PackRoot)) {
    $display = $_.File.Substring($PackRoot.Length).TrimStart('\')
  }
  Write-Host ("{0}`t{1}/{2}" -f $display, $_.Bytes, $_.Limit)
}
Write-Host 'validation ok'
