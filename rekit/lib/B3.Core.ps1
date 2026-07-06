function New-RekitBoardTimestamp {
  return (Get-Date -Format 'yyyyMMdd-HHmmssfff')
}

function New-RekitIsoTime {
  return (Get-Date).ToUniversalTime().ToString('o')
}

function Get-RekitTextHash {
  param([AllowEmptyString()][string]$Text)
  $sha = [System.Security.Cryptography.SHA256]::Create()
  try {
    $bytes = [System.Text.Encoding]::UTF8.GetBytes($Text)
    return ([System.BitConverter]::ToString($sha.ComputeHash($bytes))).Replace('-', '').ToLowerInvariant()
  } finally {
    $sha.Dispose()
  }
}

function ConvertTo-RekitJsonLine {
  param([Parameter(Mandatory=$true)]$Object)
  return ($Object | ConvertTo-Json -Depth 16 -Compress)
}

function Write-RekitJsonFile {
  param(
    [Parameter(Mandatory=$true)][string]$Path,
    [Parameter(Mandatory=$true)]$Object
  )
  Ensure-RekitDirectory (Split-Path -Parent $Path)
  [System.IO.File]::WriteAllText($Path, ($Object | ConvertTo-Json -Depth 16), [System.Text.UTF8Encoding]::new($false))
}

function Read-RekitJsonFile {
  param([Parameter(Mandatory=$true)][string]$Path)
  if (-not (Test-Path -LiteralPath $Path)) { return $null }
  $text = [System.IO.File]::ReadAllText($Path, [System.Text.Encoding]::UTF8)
  if ([string]::IsNullOrWhiteSpace($text)) { return $null }
  return ($text | ConvertFrom-Json)
}

function Add-RekitJsonLine {
  param(
    [Parameter(Mandatory=$true)][string]$Path,
    [Parameter(Mandatory=$true)]$Object
  )
  Ensure-RekitDirectory (Split-Path -Parent $Path)
  [System.IO.File]::AppendAllText($Path, (ConvertTo-RekitJsonLine $Object) + "`r`n", [System.Text.UTF8Encoding]::new($false))
}

function Read-RekitJsonLines {
  param([Parameter(Mandatory=$true)][string]$Path)
  $items = @()
  if (-not (Test-Path -LiteralPath $Path)) { return $items }
  foreach ($line in [System.IO.File]::ReadLines($Path, [System.Text.Encoding]::UTF8)) {
    if ([string]::IsNullOrWhiteSpace($line)) { continue }
    try {
      $items += ($line | ConvertFrom-Json)
    } catch {
      Write-Warning "skip malformed jsonl line in $Path"
    }
  }
  return $items
}

function Split-RekitScalarList {
  param([string]$Value)
  if ([string]::IsNullOrWhiteSpace($Value)) { return @() }
  return @($Value -split '[,;]' | ForEach-Object { $_.Trim() } | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
}

function Select-RekitFirstText {
  param([object[]]$Values)
  foreach ($value in $Values) {
    if ($null -ne $value -and -not [string]::IsNullOrWhiteSpace([string]$value)) { return [string]$value }
  }
  return ''
}

function Join-RekitRelativePath {
  param(
    [Parameter(Mandatory=$true)][string]$Root,
    [Parameter(Mandatory=$true)][string]$Path
  )
  $rootFull = [System.IO.Path]::GetFullPath($Root)
  $pathFull = [System.IO.Path]::GetFullPath($Path)
  if ([string]::IsNullOrWhiteSpace($rootFull)) { return ($pathFull -replace '\\','/') }
  $rootTrimmed = $rootFull.TrimEnd([char[]]@('\','/'))
  if ($pathFull.Equals($rootTrimmed, [System.StringComparison]::OrdinalIgnoreCase)) { return '.' }
  $prefix = $rootTrimmed + [System.IO.Path]::DirectorySeparatorChar
  if ($pathFull.StartsWith($prefix, [System.StringComparison]::OrdinalIgnoreCase)) {
    return (($pathFull.Substring($prefix.Length)) -replace '\\','/')
  }
  # Fallback: not under root, return full path normalized.
  return ($pathFull -replace '\\','/')
}
