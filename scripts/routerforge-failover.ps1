param(
    [ValidateSet('STATUS','PROMOTE_FORGEJO','RESTORE_GITHUB')]
    [string]$Mode = 'STATUS',

    [string]$Confirm = '',

    [switch]$SyncPaused
)

$ErrorActionPreference = 'Stop'

$GitHubUrl = 'https://github.com/Fifth-Ace/routerforge.git'
$ForgejoUrl = 'ssh://sc-forgejo@git.fifthace.ru:22222/FifthAce/routerforge.git'
$StateFile = '.git/routerforge-failover-state.json'

function Fail {
    param([string]$Message)
    throw $Message
}

function Invoke-GitNative {
    param([string[]]$Arguments)

    $oldPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'

    try {
        $output = @(& git @Arguments 2>&1)
        $code = $LASTEXITCODE
    }
    finally {
        $ErrorActionPreference = $oldPreference
    }

    if ($code -ne 0) {
        $joined = ($output -join ' | ')
        Fail ("git " + ($Arguments -join ' ') + " rc=" + $code + " " + $joined)
    }

    return @($output)
}

function Try-FetchRemote {
    param([string]$Remote)

    $oldPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'

    try {
        & git fetch $Remote --prune 2>&1 | Out-Host
        $code = $LASTEXITCODE
    }
    finally {
        $ErrorActionPreference = $oldPreference
    }

    return ($code -eq 0)
}

function Get-RefSha {
    param([string]$RefName)

    $oldPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'

    try {
        $value = & git rev-parse $RefName 2>$null
        $code = $LASTEXITCODE
    }
    finally {
        $ErrorActionPreference = $oldPreference
    }

    if ($code -ne 0) {
        return ''
    }

    return ([string]$value).Trim()
}

function Get-GitConfig {
    param([string]$Key)

    $oldPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'

    try {
        $value = & git config --get $Key 2>$null
        $code = $LASTEXITCODE
    }
    finally {
        $ErrorActionPreference = $oldPreference
    }

    if ($code -ne 0) {
        return ''
    }

    return ([string]$value).Trim()
}

function Set-GitConfig {
    param(
        [string]$Key,
        [string]$Value
    )

    if ([string]::IsNullOrWhiteSpace($Value)) {
        $oldPreference = $ErrorActionPreference
        $ErrorActionPreference = 'Continue'

        try {
            & git config --unset-all $Key 2>$null
            $code = $LASTEXITCODE
        }
        finally {
            $ErrorActionPreference = $oldPreference
        }

        if (($code -ne 0) -and ($code -ne 5)) {
            Fail ("unset " + $Key + " rc=" + $code)
        }

        return
    }

    Invoke-GitNative -Arguments @('config', $Key, $Value) | Out-Null
}

function Load-State {
    if (-not (Test-Path -LiteralPath $StateFile)) {
        return $null
    }

    $json = [IO.File]::ReadAllText($StateFile)
    return ($json | ConvertFrom-Json)
}

function Save-State {
    param($State)

    $encoding = New-Object System.Text.UTF8Encoding($false)
    $json = ($State | ConvertTo-Json -Depth 8) + "`n"
    [IO.File]::WriteAllText($StateFile, $json, $encoding)
}

function Assert-Repo {
    if (-not (Test-Path -LiteralPath '.git')) {
        Fail 'run from RouterForge repo root'
    }

    $origin = ([string](Invoke-GitNative -Arguments @('remote','get-url','origin'))).Trim()
    $forgejo = ([string](Invoke-GitNative -Arguments @('remote','get-url','forgejo'))).Trim()

    if ($origin -ne $GitHubUrl) {
        Fail ("origin mismatch: " + $origin)
    }

    if ($forgejo -ne $ForgejoUrl) {
        Fail ("forgejo mismatch: " + $forgejo)
    }

    $dirty = @(Invoke-GitNative -Arguments @('status','--porcelain'))
    if ($dirty.Count -ne 0) {
        Fail 'worktree/index must be clean'
    }
}

function Show-Status {
    $githubReachable = Try-FetchRemote -Remote 'origin'
    $forgejoReachable = Try-FetchRemote -Remote 'forgejo'

    $originMain = Get-RefSha -RefName 'refs/remotes/origin/main'
    $originDev = Get-RefSha -RefName 'refs/remotes/origin/dev'
    $forgejoMain = Get-RefSha -RefName 'refs/remotes/forgejo/main'
    $forgejoDev = Get-RefSha -RefName 'refs/remotes/forgejo/dev'
    $state = Load-State

    $mainParity = 'NO'
    if ((-not [string]::IsNullOrWhiteSpace($originMain)) -and ($originMain -eq $forgejoMain)) {
        $mainParity = 'PASS'
    }

    $devParity = 'NO'
    if ((-not [string]::IsNullOrWhiteSpace($originDev)) -and ($originDev -eq $forgejoDev)) {
        $devParity = 'PASS'
    }

    Write-Host ''
    Write-Host '=== STATUS ===' -ForegroundColor Cyan
    Write-Host ('GITHUB_REACHABLE=' + $(if ($githubReachable) { 'YES' } else { 'NO' }))
    Write-Host ('FORGEJO_REACHABLE=' + $(if ($forgejoReachable) { 'YES' } else { 'NO' }))
    Write-Host ('MAIN_PARITY=' + $mainParity)
    Write-Host ('DEV_PARITY=' + $devParity)
    Write-Host ('PUSH_DEFAULT=' + (Get-GitConfig -Key 'remote.pushDefault'))
    Write-Host ('DEV_TRACKING=' + (Get-GitConfig -Key 'branch.dev.remote'))
    Write-Host ('MAIN_TRACKING=' + (Get-GitConfig -Key 'branch.main.remote'))
    Write-Host ('FAILOVER_STATE=' + $(if ($null -ne $state) { 'PRESENT' } else { 'ABSENT' }))
}

function Promote-Forgejo {
    if ($Confirm -ne 'PROMOTE_FORGEJO') {
        Fail 'PROMOTE_FORGEJO requires -Confirm PROMOTE_FORGEJO'
    }

    if (-not $SyncPaused) {
        Fail 'PROMOTE_FORGEJO requires -SyncPaused after the GitHub->Forgejo sync job has been paused externally'
    }

    if ($null -ne (Load-State)) {
        Fail 'failover state already exists'
    }

    if (-not (Try-FetchRemote -Remote 'forgejo')) {
        Fail 'Forgejo fetch failed'
    }

    $githubReachable = Try-FetchRemote -Remote 'origin'

    $originMain = Get-RefSha -RefName 'refs/remotes/origin/main'
    $originDev = Get-RefSha -RefName 'refs/remotes/origin/dev'
    $forgejoMain = Get-RefSha -RefName 'refs/remotes/forgejo/main'
    $forgejoDev = Get-RefSha -RefName 'refs/remotes/forgejo/dev'

    if ([string]::IsNullOrWhiteSpace($forgejoMain) -or [string]::IsNullOrWhiteSpace($forgejoDev)) {
        Fail 'Forgejo main/dev missing'
    }

    if ([string]::IsNullOrWhiteSpace($originMain) -or [string]::IsNullOrWhiteSpace($originDev)) {
        Fail 'cached GitHub main/dev refs missing; promotion refused'
    }

    if (($originMain -ne $forgejoMain) -or ($originDev -ne $forgejoDev)) {
        Fail 'GitHub/Forgejo divergence detected; promotion refused'
    }

    $state = [ordered]@{
        schema_version = 1
        mode = 'forgejo-primary-local'
        promoted_at = [DateTime]::UtcNow.ToString('o')
        github_reachable = $githubReachable
        previous_push_default = Get-GitConfig -Key 'remote.pushDefault'
        previous_dev_remote = Get-GitConfig -Key 'branch.dev.remote'
        previous_dev_merge = Get-GitConfig -Key 'branch.dev.merge'
        previous_main_remote = Get-GitConfig -Key 'branch.main.remote'
        previous_main_merge = Get-GitConfig -Key 'branch.main.merge'
        origin_main = $originMain
        origin_dev = $originDev
        forgejo_main = $forgejoMain
        forgejo_dev = $forgejoDev
    }

    Save-State -State $state

    try {
        Invoke-GitNative -Arguments @('config','remote.pushDefault','forgejo') | Out-Null
        Invoke-GitNative -Arguments @('config','branch.dev.remote','forgejo') | Out-Null
        Invoke-GitNative -Arguments @('config','branch.dev.merge','refs/heads/dev') | Out-Null
        Invoke-GitNative -Arguments @('config','branch.main.remote','forgejo') | Out-Null
        Invoke-GitNative -Arguments @('config','branch.main.merge','refs/heads/main') | Out-Null
    }
    catch {
        Set-GitConfig -Key 'remote.pushDefault' -Value ([string]$state.previous_push_default)
        Set-GitConfig -Key 'branch.dev.remote' -Value ([string]$state.previous_dev_remote)
        Set-GitConfig -Key 'branch.dev.merge' -Value ([string]$state.previous_dev_merge)
        Set-GitConfig -Key 'branch.main.remote' -Value ([string]$state.previous_main_remote)
        Set-GitConfig -Key 'branch.main.merge' -Value ([string]$state.previous_main_merge)
        Remove-Item -LiteralPath $StateFile -Force -ErrorAction SilentlyContinue
        throw
    }

    Write-Host 'RESULT=PASS'
    Write-Host 'PRIMARY=FORGEJO_LOCAL'
    Write-Host 'REVERSE_SYNC=NONE'
    Write-Host 'GITHUB_REFS_MUTATED=NO'
    Write-Host 'FORGEJO_REFS_MUTATED=NO'
}

function Restore-GitHub {
    if ($Confirm -ne 'RESTORE_GITHUB') {
        Fail 'RESTORE_GITHUB requires -Confirm RESTORE_GITHUB'
    }

    $state = Load-State
    if ($null -eq $state) {
        Fail 'no failover state exists'
    }

    if (-not (Try-FetchRemote -Remote 'origin')) {
        Fail 'GitHub is not reachable; restore refused'
    }

    if (-not (Try-FetchRemote -Remote 'forgejo')) {
        Fail 'Forgejo is not reachable; restore refused'
    }

    $originMain = Get-RefSha -RefName 'refs/remotes/origin/main'
    $originDev = Get-RefSha -RefName 'refs/remotes/origin/dev'
    $forgejoMain = Get-RefSha -RefName 'refs/remotes/forgejo/main'
    $forgejoDev = Get-RefSha -RefName 'refs/remotes/forgejo/dev'

    if (($originMain -ne $forgejoMain) -or ($originDev -ne $forgejoDev)) {
        Write-Host 'REVERSE_SYNC=NOT_PERFORMED'
        Write-Host 'MANUAL_RECONCILIATION_REQUIRED=YES'
        Fail 'automatic Forgejo->GitHub reconciliation is forbidden'
    }

    Set-GitConfig -Key 'remote.pushDefault' -Value ([string]$state.previous_push_default)
    Set-GitConfig -Key 'branch.dev.remote' -Value ([string]$state.previous_dev_remote)
    Set-GitConfig -Key 'branch.dev.merge' -Value ([string]$state.previous_dev_merge)
    Set-GitConfig -Key 'branch.main.remote' -Value ([string]$state.previous_main_remote)
    Set-GitConfig -Key 'branch.main.merge' -Value ([string]$state.previous_main_merge)

    Remove-Item -LiteralPath $StateFile -Force

    Write-Host 'RESULT=PASS'
    Write-Host 'PRIMARY=GITHUB'
    Write-Host 'REVERSE_SYNC=NONE'
    Write-Host 'REF_PARITY=PASS'
    Write-Host 'GITHUB_REFS_MUTATED=NO'
    Write-Host 'FORGEJO_REFS_MUTATED=NO'
}

Assert-Repo

switch ($Mode) {
    'STATUS' {
        Show-Status
    }
    'PROMOTE_FORGEJO' {
        Promote-Forgejo
    }
    'RESTORE_GITHUB' {
        Restore-GitHub
    }
}
