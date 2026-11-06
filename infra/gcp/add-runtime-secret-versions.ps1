[CmdletBinding()]
param(
  [string]$Project = "generated-mote-467410-b0",
  [string]$EnvPath,
  [switch]$RefreshProviderKeys
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($EnvPath)) {
  $EnvPath = Join-Path $PSScriptRoot "..\..\.env"
}

if (-not (Test-Path -LiteralPath $EnvPath)) {
  throw "Không tìm thấy file env: $EnvPath"
}

$ResolvedEnvPath = (Resolve-Path -LiteralPath $EnvPath).Path
$EnvValues = @{}
foreach ($Line in Get-Content -LiteralPath $ResolvedEnvPath) {
  if ($Line -match '^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=(.*)$') {
    $Key = $Matches[1]
    $Value = $Matches[2].Trim()
    if ($Value.Length -ge 2 -and
        (($Value.StartsWith('"') -and $Value.EndsWith('"')) -or
         ($Value.StartsWith("'") -and $Value.EndsWith("'")))) {
      $Value = $Value.Substring(1, $Value.Length - 2)
    }
    $EnvValues[$Key] = $Value
  }
}

$Mappings = @(
  @{ Secret = "goshopx-jwt-secret"; Env = "SECRET_KEY" },
  @{ Secret = "goshopx-minio-access-key"; Env = "MINIO_ACCESS_KEY" },
  @{ Secret = "goshopx-minio-secret-key"; Env = "MINIO_SECRET_KEY" },
  @{ Secret = "goshopx-google-client-id"; Env = "GOOGLE_CLIENT_ID" },
  @{ Secret = "goshopx-vnpay-tmn-code"; Env = "VNPAY_TMN_CODE" },
  @{ Secret = "goshopx-vnpay-hash-secret"; Env = "VNPAY_HASH_SECRET" },
  @{ Secret = "goshopx-vnpay-payment-url"; Env = "VNPAY_PAYMENT_URL" },
  @{ Secret = "goshopx-gmail-username"; Env = "GMAIL_USERNAME" },
  @{ Secret = "goshopx-smtp-password"; Env = "GMAIL_PASSWORD" },
  @{ Secret = "goshopx-gmail-from"; Env = "GMAIL_FROM" },
  @{ Secret = "goshopx-deepseek-api-key"; Env = "DEEPSEEK_API_KEY"; FallbackEnv = "AI_API_KEY" },
  @{ Secret = "goshopx-gemini-api-key"; Env = "GEMINI_API_KEY" },
  @{ Secret = "goshopx-tavily-api-key"; Env = "TAVILY_API_KEY" },
  @{ Secret = "goshopx-typesafe-api-key"; Env = "TYPESAFE_API_KEY" },
  @{ Secret = "goshopx-langsmith-api-key"; Env = "LANGSMITH_API_KEY" }
)
$ProviderSecrets = @(
  "goshopx-deepseek-api-key", "goshopx-gemini-api-key",
  "goshopx-tavily-api-key", "goshopx-typesafe-api-key", "goshopx-langsmith-api-key"
)

$NeedsInput = @()
foreach ($Mapping in $Mappings) {
  $Value = [string]$EnvValues[$Mapping.Env]
  if ([string]::IsNullOrWhiteSpace($Value) -and $Mapping.FallbackEnv) {
    $Value = [string]$EnvValues[$Mapping.FallbackEnv]
  }

  if ([string]::IsNullOrWhiteSpace($Value) -or
      $Value -match '(?i)^(change-me|replace|your[-_]|<.*>)') {
    $NeedsInput += "$($Mapping.Secret) (env: $($Mapping.Env))"
    continue
  }

  $LatestStateOutput = @(gcloud secrets versions list $Mapping.Secret `
    --project $Project `
    --sort-by="~createTime" `
    --limit=1 `
    --format="value(state)" 2>$null)
  $GcloudExitCode = $LASTEXITCODE
  if ($GcloudExitCode -ne 0) {
    throw "Không kiểm tra được version của $($Mapping.Secret) trong project $Project"
  }

  $LatestState = ([string]::Join(",", $LatestStateOutput)).Trim()
  if ($LatestState -eq "enabled" -and (-not $RefreshProviderKeys -or $Mapping.Secret -notin $ProviderSecrets)) {
    Write-Host "Đã có version enabled, bỏ qua: $($Mapping.Secret)"
    continue
  }

  $TempFile = [IO.Path]::GetTempFileName()
  try {
    [IO.File]::WriteAllBytes($TempFile, [Text.Encoding]::UTF8.GetBytes($Value))
    gcloud secrets versions add $Mapping.Secret `
      --project $Project `
      --data-file $TempFile
    if ($LASTEXITCODE -ne 0) {
      throw "Thêm version thất bại: $($Mapping.Secret)"
    }
    Write-Host "Đã thêm version: $($Mapping.Secret)"
  }
  finally {
    if (Test-Path -LiteralPath $TempFile) {
      Remove-Item -LiteralPath $TempFile -Force
    }
  }
}

if ($NeedsInput.Count -gt 0) {
  Write-Host "Cần điền giá trị thật, không phải placeholder, cho các mục sau:"
  $NeedsInput | ForEach-Object { Write-Host " - $_" }
} else {
  Write-Host "Đã xử lý xong các runtime secret còn thiếu từ .env."
}
