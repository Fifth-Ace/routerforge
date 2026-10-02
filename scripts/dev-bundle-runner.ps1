param(
    [string]$BundleRoot = $PSScriptRoot
)

$ErrorActionPreference = "Stop"

function Fail([string]$Reason) {
    throw $Reason
}

function RemoteBranchSha([string]$Repo, [string]$Branch) {
    $url = "https://github.com/$Repo.git"
    for ($try = 1; $try -le 12; $try++) {
        $old = $ErrorActionPreference
        $ErrorActionPreference = "Continue"
        $raw = @(& git ls-remote $url "refs/heads/$Branch" 2>&1)
        $rc = $LASTEXITCODE
        $ErrorActionPreference = $old
        if ($rc -eq 0 -and $raw.Count -ge 1) {
            $line = [string]$raw[0]
            $parts = $line -split "\s+"
            if ($parts.Count -ge 1 -and $parts[0] -match "^[0-9a-fA-F]{40}$") {
                return $parts[0].ToLowerInvariant()
            }
        }
        if ($try -lt 12) {
            Write-Host "GIT_REMOTE_RETRY: $try/12" -ForegroundColor Yellow
            Start-Sleep -Seconds 3
        }
    }
    Fail "REMOTE_BRANCH_READ_FAIL"
}

function WaitExactPushRun([string]$Repo, [string]$Branch, [string]$Sha) {
    for ($i = 1; $i -le 120; $i++) {
        $old = $ErrorActionPreference
        $ErrorActionPreference = "Continue"
        $raw = @(& gh run list --repo $Repo --workflow ci.yml --branch $Branch --event push --commit $Sha --limit 20 --json databaseId,headSha,headBranch,status,conclusion 2>&1)
        $rc = $LASTEXITCODE
        $ErrorActionPreference = $old

        if ($rc -eq 0) {
            try {
                $runs = @(($raw -join "`n") | ConvertFrom-Json)
                $exact = @($runs | Where-Object {
                    [string]$_.headSha -eq $Sha -and [string]$_.headBranch -eq $Branch
                })
                if ($exact.Count -eq 1) { return [string]$exact[0].databaseId }
                if ($exact.Count -gt 1) { Fail "MULTIPLE_EXACT_PUSH_RUNS" }
            } catch {
                if ($_.Exception.Message -eq "MULTIPLE_EXACT_PUSH_RUNS") { throw }
            }
        } else {
            Write-Host "GH_RUN_LOOKUP_RETRY: $i/120" -ForegroundColor Yellow
        }
        Start-Sleep -Seconds 3
    }
    Fail "EXACT_PUSH_RUN_NOT_FOUND"
}

try {
    $manifestPath = Join-Path $BundleRoot "manifest.json"
    $patchPath = Join-Path $BundleRoot "change.patch"

    if (-not (Test-Path -LiteralPath $manifestPath)) {
        Fail "MANIFEST_NOT_FOUND"
    }
    if (-not (Test-Path -LiteralPath $patchPath)) {
        Fail "PATCH_NOT_FOUND"
    }

    $manifest = Get-Content -Raw -LiteralPath $manifestPath | ConvertFrom-Json
    $repo = [string]$manifest.repo
    $branch = [string]$manifest.branch
    $base = [string]$manifest.base_sha
    $message = [string]$manifest.commit_message
    $expectedFiles = @($manifest.files | ForEach-Object { [string]$_ } | Sort-Object)

    if (-not $repo -or -not $branch -or -not $base -or -not $message -or $expectedFiles.Count -eq 0) {
        Fail "MANIFEST_INVALID"
    }

    Write-Host "=== PRECHECK ===" -ForegroundColor Cyan
    Write-Host "DELIVERY: V2-R2"
    Write-Host "REPO: $repo"
    Write-Host "BRANCH: $branch"
    Write-Host "BASE: $base"

    & git --version | Out-Host
    if ($LASTEXITCODE -ne 0) { Fail "GIT_NOT_AVAILABLE" }

    & gh auth status 2>&1 | Out-Host
    if ($LASTEXITCODE -ne 0) { Fail "GH_AUTH_FAIL" }

    $remoteBefore = RemoteBranchSha $repo $branch
    if ($remoteBefore -ne $base) {
        Fail "REMOTE_BASE_DRIFT"
    }

    $root = "C:\temp"
    if (-not (Test-Path -LiteralPath $root)) {
        New-Item -ItemType Directory -Path $root -Force | Out-Null
    }

    $work = Join-Path $root "routerforge-dev-bundle-work"
    if (Test-Path -LiteralPath $work) {
        Remove-Item -LiteralPath $work -Recurse -Force
    }

    Write-Host "PRECHECK: PASS" -ForegroundColor Green

    Write-Host ""
    Write-Host "=== ACTION ===" -ForegroundColor Cyan

    & git -c core.autocrlf=false clone --branch $branch --single-branch "https://github.com/$repo.git" $work
    if ($LASTEXITCODE -ne 0) { Fail "CLONE_FAIL" }

    Set-Location $work

    & git config core.autocrlf false
    & git config gc.auto 0
    & git config maintenance.auto false
    & git config user.name "FifthAce"
    & git config user.email "248300012+Fifth-Ace@users.noreply.github.com"

    $localBase = (& git rev-parse HEAD).Trim()
    if ($localBase -ne $base) {
        Fail "LOCAL_BASE_MISMATCH"
    }

    if (@(& git status --porcelain).Count -ne 0) {
        Fail "CLONE_NOT_CLEAN"
    }

    & git apply --check --whitespace=error-all $patchPath
    if ($LASTEXITCODE -ne 0) {
        Fail "PATCH_CHECK_FAIL"
    }

    & git apply --index --whitespace=error-all $patchPath
    if ($LASTEXITCODE -ne 0) {
        Fail "PATCH_APPLY_FAIL"
    }

    & git diff --cached --check
    if ($LASTEXITCODE -ne 0) {
        Fail "STAGED_DIFF_CHECK_FAIL"
    }

    $actualFiles = @(& git diff --cached --name-only | Sort-Object)
    if (($actualFiles -join "`n") -ne ($expectedFiles -join "`n")) {
        Write-Host "EXPECTED:"
        $expectedFiles | ForEach-Object { Write-Host "  $_" }
        Write-Host "ACTUAL:"
        $actualFiles | ForEach-Object { Write-Host "  $_" }
        Fail "FILE_SCOPE_MISMATCH"
    }

    Write-Host "PATCH_APPLY: PASS" -ForegroundColor Green
    Write-Host "FILES: $($actualFiles.Count)"

    & git commit -m $message
    if ($LASTEXITCODE -ne 0) {
        Fail "COMMIT_FAIL"
    }

    $sha = (& git rev-parse HEAD).Trim()
    Write-Host "COMMIT: $sha"

    $remoteBeforePush = RemoteBranchSha $repo $branch
    if ($remoteBeforePush -ne $base) {
        Fail "REMOTE_MOVED_BEFORE_PUSH"
    }

    & git push origin "HEAD:$branch"
    $pushRc = $LASTEXITCODE

    if ($pushRc -ne 0) {
        $remoteAfterPushError = RemoteBranchSha $repo $branch
        if ($remoteAfterPushError -ne $sha) {
            Fail "PUSH_FAIL"
        }
        Write-Host "PUSH_TRANSPORT_ERROR_BUT_REMOTE_MATCHES: CONTINUE" -ForegroundColor Yellow
    }

    $remoteAfterPush = RemoteBranchSha $repo $branch
    if ($remoteAfterPush -ne $sha) {
        Fail "REMOTE_PUSH_VERIFY_FAIL"
    }

    Write-Host "PUSH: PASS" -ForegroundColor Green

    Write-Host ""
    Write-Host "=== LIVE ===" -ForegroundColor Cyan

    $runId = WaitExactPushRun $repo $branch $sha
    Write-Host "RUN_ID: $runId"
    Write-Host "RUN_SHA: $sha"

    $old = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    & gh run watch $runId --repo $repo --exit-status
    $watchRc = $LASTEXITCODE
    $ErrorActionPreference = $old

    if ($watchRc -ne 0) {
        Write-Host ""
        Write-Host "=== FAILED LOG ===" -ForegroundColor Red
        $old = $ErrorActionPreference
        $ErrorActionPreference = "Continue"
        & gh run view $runId --repo $repo --log-failed
        $ErrorActionPreference = $old
        Fail "CI_RED"
    }

    Write-Host ""
    Write-Host "=== VERIFY ===" -ForegroundColor Cyan

    $runRaw = @(& gh run view $runId --repo $repo --json databaseId,headSha,headBranch,status,conclusion,event 2>&1)
    if ($LASTEXITCODE -ne 0) { Fail "CI_RUN_VERIFY_FAIL" }
    try { $run = ($runRaw -join "`n") | ConvertFrom-Json }
    catch { Fail "CI_RUN_VERIFY_JSON_FAIL" }

    if ([string]$run.headSha -ne $sha) { Fail "CI_SHA_MISMATCH" }
    if ([string]$run.status -ne "completed") { Fail "CI_NOT_COMPLETED" }
    if ([string]$run.conclusion -ne "success") { Fail "CI_NOT_SUCCESS" }

    $remoteFinal = RemoteBranchSha $repo $branch
    if ($remoteFinal -ne $sha) {
        Fail "REMOTE_FINAL_SHA_MISMATCH"
    }

    Write-Host "CI_EXACT_SHA: PASS" -ForegroundColor Green
    Write-Host "CI_CONCLUSION: PASS" -ForegroundColor Green
    Write-Host "REMOTE_BRANCH: PASS" -ForegroundColor Green

    Set-Location $root
    Remove-Item -LiteralPath $work -Recurse -Force

    Write-Host ""
    Write-Host "=== RESULT ===" -ForegroundColor Green
    Write-Host "STATE: PASS" -ForegroundColor Green
    Write-Host "COMMIT: $sha"
    Write-Host "CI_RUN_ID: $runId"
    Write-Host "BRANCH: $branch"
    Write-Host "INTERACTIVE_SHELL: PRESERVED"
}
catch {
    Write-Host ""
    Write-Host "=== RESULT ===" -ForegroundColor Red
    Write-Host "STATE: FAIL" -ForegroundColor Red
    Write-Host "REASON: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host "INTERACTIVE_SHELL: PRESERVED"
    throw
}
