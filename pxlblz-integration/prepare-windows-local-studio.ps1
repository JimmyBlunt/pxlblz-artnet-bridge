param(
  [Parameter(Mandatory=$true)]
  [string]$PxlblzPath,

  [Parameter(Mandatory=$true)]
  [string]$MainPxlblzPath
)

$ErrorActionPreference = "Stop"

function Resolve-Directory([string]$PathValue, [string]$Label) {
  if (!(Test-Path $PathValue -PathType Container)) {
    throw "$Label directory not found: $PathValue"
  }
  return (Resolve-Path $PathValue).Path
}

$worktree = Resolve-Directory $PxlblzPath "PXLBLZ Art-Net worktree"
$main = Resolve-Directory $MainPxlblzPath "PXLBLZ main worktree"

$worktreePackage = Join-Path $worktree "package.json"
$mainPackage = Join-Path $main "package.json"
if (!(Test-Path $worktreePackage) -or !(Test-Path $mainPackage)) {
  throw "Both paths must be PXLBLZ-IDE checkouts containing package.json."
}

$mainBranch = (& git -C $main branch --show-current).Trim()
if ($LASTEXITCODE -ne 0) { throw "Could not inspect main worktree branch." }
if ($mainBranch -ne "main") {
  throw "MainPxlblzPath must be on branch main; found '$mainBranch'."
}

$mainDevVars = Join-Path $main ".dev.vars"
$example = Join-Path $main ".dev.vars.example"
if (!(Test-Path $mainDevVars)) {
  if (!(Test-Path $example)) { throw ".dev.vars.example not found in main worktree." }
  Copy-Item $example $mainDevVars
}

$vars = Get-Content $mainDevVars -Raw
$needsSecret = $vars -match '(?m)^SESSION_SECRET\s*=\s*$' -or
               $vars -match '(?m)^SESSION_SECRET\s*=\s*replace-with-a-long-local-secret\s*$'
if ($needsSecret) {
  $bytes = New-Object byte[] 48
  $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
  try { $rng.GetBytes($bytes) } finally { $rng.Dispose() }
  $secret = [Convert]::ToBase64String($bytes)
  $vars = [regex]::Replace($vars, '(?m)^SESSION_SECRET\s*=.*$', "SESSION_SECRET=$secret")
  Set-Content -Path $mainDevVars -Value $vars -Encoding utf8
}

$worktreeDevVars = Join-Path $worktree ".dev.vars"
Copy-Item $mainDevVars $worktreeDevVars -Force

Push-Location $worktree
try {
  Write-Host "Applying PXLBLZ local D1 migrations..."
  & npm run db:migrate:local
  if ($LASTEXITCODE -ne 0) { throw "PXLBLZ local D1 migration failed." }

  $seedSql = "INSERT INTO users (id, github_user_id, github_login, display_name, avatar_url, created_at, updated_at) VALUES ('github:local-dev','local-dev','local-dev','Local Dev',NULL,unixepoch(),unixepoch()) ON CONFLICT(id) DO UPDATE SET display_name=excluded.display_name, updated_at=excluded.updated_at;"
  Write-Host "Provisioning synthetic local developer identity..."
  & npx wrangler d1 execute pxlblz-ide --local --command $seedSql
  if ($LASTEXITCODE -ne 0) { throw "Could not provision local developer identity." }

  Write-Host "Minting synthetic developer session..."
  $sessionOutput = (& npm run dev:session -- --developer 2>&1 | Out-String)
  if ($LASTEXITCODE -ne 0) { throw "Could not mint local developer session. Output:`n$sessionOutput" }

  $match = [regex]::Match($sessionOutput, '(?m)^pxlblz_session=(.+)$')
  if (!$match.Success) { throw "Session helper did not return pxlblz_session." }
  $token = $match.Groups[1].Value.Trim()

  $sessionFile = Join-Path $worktree ".pxlblz-local-session.txt"
  Set-Content -Path $sessionFile -Value "pxlblz_session=$token" -Encoding ascii

  Write-Host ""
  Write-Host "LOCAL_STUDIO_PREP_PASS"
  Write-Host "Session file: $sessionFile"
  Write-Host ""
  Write-Host "Next terminal:"
  Write-Host ('  cd /d "' + $worktree + '"')
  Write-Host "  npm run dev"
  Write-Host ""
  Write-Host "Open:"
  Write-Host "  http://localhost:5174/PXLBLZ-IDE/studio?pxout=1"
  Write-Host ""
  Write-Host "Browser Console cookie command:"
  Write-Host ('  document.cookie = "pxlblz_session=' + $token + '; path=/; SameSite=Lax"')
  Write-Host ""
  Write-Host "Then verify:"
  Write-Host "  fetch('/api/me').then(r => r.json()).then(console.log)"
} finally {
  Pop-Location
}
