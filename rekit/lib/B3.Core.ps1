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

function Get-RekitObjectValue {
  param(
    $Object,
    [Parameter(Mandatory=$true)][string]$Name
  )
  if ($null -eq $Object) { return $null }
  if ($Object -is [System.Collections.IDictionary]) {
    if ($Object.Contains($Name)) { return $Object[$Name] }
    return $null
  }
  $prop = $Object.PSObject.Properties[$Name]
  if ($null -ne $prop) { return $prop.Value }
  return $null
}

function Format-RekitScalarDisplay {
  param($Value)
  if ($null -eq $Value) { return '' }
  if ($Value -is [string]) { return [string]$Value }
  if ($Value -is [System.Collections.IEnumerable]) {
    $items = @()
    foreach ($item in $Value) {
      if ($null -ne $item -and -not [string]::IsNullOrWhiteSpace([string]$item)) { $items += [string]$item }
    }
    return ($items -join ',')
  }
  return [string]$Value
}

function Format-RekitGateRequestDetail {
  param(
    [Parameter(Mandatory=$true)]$Event,
    [switch]$OmitStatus,
    [switch]$OmitBatch
  )
  $parts = New-Object System.Collections.Generic.List[string]
  $status = Format-RekitScalarDisplay (Get-RekitObjectValue -Object $Event -Name 'status')
  if (-not $OmitStatus -and -not [string]::IsNullOrWhiteSpace($status)) { $parts.Add("status=$status") }
  $actor = Format-RekitScalarDisplay (Get-RekitObjectValue -Object $Event -Name 'actor')
  if (-not [string]::IsNullOrWhiteSpace($actor)) { $parts.Add("by=$actor") }
  $risk = Format-RekitScalarDisplay (Get-RekitObjectValue -Object $Event -Name 'risk')
  if (-not [string]::IsNullOrWhiteSpace($risk)) { $parts.Add("risk=$risk") }
  $target = Format-RekitScalarDisplay (Get-RekitObjectValue -Object $Event -Name 'target')
  if (-not [string]::IsNullOrWhiteSpace($target)) { $parts.Add("target=$target") }
  $batch = Format-RekitScalarDisplay (Get-RekitObjectValue -Object $Event -Name 'batchId')
  if (-not $OmitBatch -and -not [string]::IsNullOrWhiteSpace($batch)) { $parts.Add("batch=$batch") }
  $gate = Get-RekitObjectValue -Object $Event -Name 'gate'
  if ($null -ne $gate) {
    $action = Format-RekitScalarDisplay (Get-RekitObjectValue -Object $gate -Name 'action')
    if (-not [string]::IsNullOrWhiteSpace($action)) { $parts.Add("action=$action") }
    $scope = Format-RekitScalarDisplay (Get-RekitObjectValue -Object $gate -Name 'scope')
    if (-not [string]::IsNullOrWhiteSpace($scope)) { $parts.Add("scope=$scope") }
    $budget = Format-RekitScalarDisplay (Get-RekitObjectValue -Object $gate -Name 'budget')
    if (-not [string]::IsNullOrWhiteSpace($budget)) { $parts.Add("budget=$budget") }
    $tried = Format-RekitScalarDisplay (Get-RekitObjectValue -Object $gate -Name 'triedLightSteps')
    if (-not [string]::IsNullOrWhiteSpace($tried)) { $parts.Add("tried=$tried") }
    $stop = Format-RekitScalarDisplay (Get-RekitObjectValue -Object $gate -Name 'stopConditions')
    if (-not [string]::IsNullOrWhiteSpace($stop)) { $parts.Add("stop=$stop") }
  }
  if ($parts.Count -eq 0) { return '' }
  return ' | ' + ($parts -join ' | ')
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
