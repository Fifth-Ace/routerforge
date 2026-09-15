param(
  [ValidateSet('STATUS','PROMOTE_FORGEJO','RESTORE_GITHUB')][string]$Mode='STATUS',
  [string]$Confirm='',
  [switch]$SyncPaused
)
$ErrorActionPreference='Stop'
$GitHub='https://github.com/Fifth-Ace/routerforge.git'
$Forgejo='ssh://sc-forgejo@git.fifthace.ru:22222/FifthAce/routerforge.git'
$StateFile='.git/routerforge-failover-state.json'
function Fail([string]$M){throw $M}
function Git([string[]]$A){$o=$ErrorActionPreference;$ErrorActionPreference='Continue';$r=& git @A 2>&1;$c=$LASTEXITCODE;$ErrorActionPreference=$o;if($c-ne0){Fail(('git '+($A-join' ')+' rc='+$c+' '+($r-join' | ')))};@($r)}
function TryFetch([string]$R){$o=$ErrorActionPreference;$ErrorActionPreference='Continue';& git fetch $R --prune 2>&1|Out-Host;$c=$LASTEXITCODE;$ErrorActionPreference=$o;return($c-eq0)}
function Sha([string]$R){$o=$ErrorActionPreference;$ErrorActionPreference='Continue';$v=& git rev-parse $R 2>$null;$c=$LASTEXITCODE;$ErrorActionPreference=$o;if($c-ne0){return''};return([string]$v).Trim()}
function Cfg([string]$K){$o=$ErrorActionPreference;$ErrorActionPreference='Continue';$v=& git config --get $K 2>$null;$c=$LASTEXITCODE;$ErrorActionPreference=$o;if($c-ne0){return''};return([string]$v).Trim()}
function SetCfg([string]$K,[string]$V){if([string]::IsNullOrWhiteSpace($V)){$o=$ErrorActionPreference;$ErrorActionPreference='Continue';& git config --unset-all $K 2>$null;$c=$LASTEXITCODE;$ErrorActionPreference=$o;if($c-ne0-and$c-ne5){Fail('unset '+$K+' rc='+$c)}}else{Git @('config',$K,$V)|Out-Null}}
function LoadState(){if(!(Test-Path $StateFile)){return $null};[IO.File]::ReadAllText($StateFile)|ConvertFrom-Json}
function SaveState($S){$u=New-Object Text.UTF8Encoding($false);[IO.File]::WriteAllText($StateFile,(($S|ConvertTo-Json -Depth 8)+"`n"),$u)}
function AssertRepo(){if(!(Test-Path '.git')){Fail 'run from RouterForge repo root'};$o=([string](Git @('remote','get-url','origin'))).Trim();$f=([string](Git @('remote','get-url','forgejo'))).Trim();if($o-ne$GitHub){Fail('origin mismatch: '+$o)};if($f-ne$Forgejo){Fail('forgejo mismatch: '+$f)};if(@(Git @('status','--porcelain')).Count-ne0){Fail 'worktree/index must be clean'}}
function Status(){
 $go=TryFetch 'origin';$fo=TryFetch 'forgejo';$om=Sha 'refs/remotes/origin/main';$od=Sha 'refs/remotes/origin/dev';$fm=Sha 'refs/remotes/forgejo/main';$fd=Sha 'refs/remotes/forgejo/dev';$s=LoadState
 Write-Host '';Write-Host '=== STATUS ===' -ForegroundColor Cyan
 Write-Host ('GITHUB_REACHABLE='+$(if($go){'YES'}else{'NO'}));Write-Host ('FORGEJO_REACHABLE='+$(if($fo){'YES'}else{'NO'}));Write-Host ('MAIN_PARITY='+$(if($om-and$om-eq$fm){'PASS'}else{'NO'}));Write-Host ('DEV_PARITY='+$(if($od-and$od-eq$fd){'PASS'}else{'NO'}));Write-Host ('PUSH_DEFAULT='+$(Cfg 'remote.pushDefault'));Write-Host ('DEV_TRACKING='+$(Cfg 'branch.dev.remote'));Write-Host ('MAIN_TRACKING='+$(Cfg 'branch.main.remote'));Write-Host ('FAILOVER_STATE='+$(if($s){'PRESENT'}else{'ABSENT'}))
}
function Promote(){
 if($Confirm-ne'PROMOTE_FORGEJO'){Fail 'PROMOTE_FORGEJO requires -Confirm PROMOTE_FORGEJO'};if(!$SyncPaused){Fail 'PROMOTE_FORGEJO requires -SyncPaused after the GitHub->Forgejo sync job has been paused externally'};if(LoadState){Fail 'failover state already exists'}
 if(!(TryFetch 'forgejo')){Fail 'Forgejo fetch failed'};$go=TryFetch 'origin';$om=Sha 'refs/remotes/origin/main';$od=Sha 'refs/remotes/origin/dev';$fm=Sha 'refs/remotes/forgejo/main';$fd=Sha 'refs/remotes/forgejo/dev';if(!$fm-or!$fd){Fail 'Forgejo main/dev missing'};if($om-ne$fm-or$od-ne$fd){Fail 'GitHub/Forgejo divergence detected; promotion refused'}
 $s=[ordered]@{schema_version=1;mode='forgejo-primary-local';promoted_at=[DateTime]::UtcNow.ToString('o');github_reachable=$go;previous_push_default=(Cfg 'remote.pushDefault');previous_dev_remote=(Cfg 'branch.dev.remote');previous_main_remote=(Cfg 'branch.main.remote');origin_main=$om;origin_dev=$od;forgejo_main=$fm;forgejo_dev=$fd};SaveState $s
 try{Git @('config','remote.pushDefault','forgejo')|Out-Null;Git @('config','branch.dev.remote','forgejo')|Out-Null;Git @('config','branch.dev.merge','refs/heads/dev')|Out-Null;Git @('config','branch.main.remote','forgejo')|Out-Null;Git @('config','branch.main.merge','refs/heads/main')|Out-Null}catch{SetCfg 'remote.pushDefault' ([string]$s.previous_push_default);SetCfg 'branch.dev.remote' ([string]$s.previous_dev_remote);SetCfg 'branch.main.remote' ([string]$s.previous_main_remote);Remove-Item $StateFile -Force -ErrorAction SilentlyContinue;throw}
 Write-Host 'RESULT=PASS';Write-Host 'PRIMARY=FORGEJO_LOCAL';Write-Host 'REVERSE_SYNC=NONE';Write-Host 'GITHUB_REFS_MUTATED=NO';Write-Host 'FORGEJO_REFS_MUTATED=NO'
}
function Restore(){
 if($Confirm-ne'RESTORE_GITHUB'){Fail 'RESTORE_GITHUB requires -Confirm RESTORE_GITHUB'};$s=LoadState;if(!$s){Fail 'no failover state exists'};if(!(TryFetch 'origin')){Fail 'GitHub is not reachable; restore refused'};if(!(TryFetch 'forgejo')){Fail 'Forgejo is not reachable; restore refused'};$om=Sha 'refs/remotes/origin/main';$od=Sha 'refs/remotes/origin/dev';$fm=Sha 'refs/remotes/forgejo/main';$fd=Sha 'refs/remotes/forgejo/dev';if($om-ne$fm-or$od-ne$fd){Write-Host 'REVERSE_SYNC=NOT_PERFORMED';Write-Host 'MANUAL_RECONCILIATION_REQUIRED=YES';Fail 'automatic Forgejo->GitHub reconciliation is forbidden'}
 SetCfg 'remote.pushDefault' ([string]$s.previous_push_default);SetCfg 'branch.dev.remote' ([string]$s.previous_dev_remote);SetCfg 'branch.main.remote' ([string]$s.previous_main_remote);Git @('config','branch.dev.merge','refs/heads/dev')|Out-Null;Git @('config','branch.main.merge','refs/heads/main')|Out-Null;Remove-Item $StateFile -Force
 Write-Host 'RESULT=PASS';Write-Host 'PRIMARY=GITHUB';Write-Host 'REVERSE_SYNC=NONE';Write-Host 'REF_PARITY=PASS';Write-Host 'GITHUB_REFS_MUTATED=NO';Write-Host 'FORGEJO_REFS_MUTATED=NO'
}
AssertRepo
switch($Mode){'STATUS'{Status};'PROMOTE_FORGEJO'{Promote};'RESTORE_GITHUB'{Restore}}