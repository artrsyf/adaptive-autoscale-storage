param(
    [ValidateSet('','constant','ramp','burst','db-lock')][string]$Scenario = '',
    [ValidateSet('','uniform','hot')][string]$Distribution = '',
    [int]$Rate = 0,
    [int]$Seconds = 0,
    [int]$ReadPercent = -1,
    [int]$Records = 0,
    [int]$WarmupSeconds = -1,
    [string]$Prometheus = 'http://127.0.0.1:9090',
    [string]$Grafana = 'http://127.0.0.1:3000'
)
$ErrorActionPreference = 'Stop'
function Invoke-StorageCompose {
    & docker compose @args
    if ($LASTEXITCODE -ne 0) { throw "Docker Compose failed (exit $LASTEXITCODE)" }
}
$projectRoot = Split-Path $PSScriptRoot -Parent
Set-Location $projectRoot


$lockJob = $null
$containerID = $null
try {
    $effectiveScenario = if ($Scenario -eq 'db-lock') { 'constant' } else { $Scenario }
    $parameters = @('--config','/etc/load/config.yaml','--rate',"$Rate",'--seconds',"$Seconds",'--read-percent',"$ReadPercent",'--records',"$Records")
    if ($effectiveScenario) { $parameters += @('--scenario',$effectiveScenario) }
    if ($Distribution) { $parameters += @('--distribution',$Distribution) }
    if ($WarmupSeconds -ge 0) { $parameters += @('--warmup',"$($WarmupSeconds)s") }
    # Inspect non-secret Compose values instead of keeping an infrastructure schema here.
    $compose = Invoke-StorageCompose config --no-interpolate --format json | ConvertFrom-Json
    $Nodes = @($compose.services.PSObject.Properties.Name | Where-Object { $_ -match '^processing-unit-' }).Count
    $endpoints = @{ prometheus=$Prometheus; grafana=$Grafana; project=$compose.name; database_user=$compose.services.postgres.environment.POSTGRES_USER; database=$compose.services.postgres.environment.POSTGRES_DB }
    if ($Scenario -eq "db-lock" -and $Seconds -gt 0 -and $Seconds -lt 30) { throw "db-lock requires at least 30 seconds of measurement." }
    Invoke-StorageCompose stop load
    Invoke-StorageCompose up -d --build --wait --remove-orphans
    $before = @(Get-ChildItem -LiteralPath results -Directory -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Name)
    Invoke-StorageCompose build load
    $containerID = Invoke-StorageCompose run -d --no-deps --use-aliases load @parameters
    $deadline = (Get-Date).AddSeconds($(if ($Seconds -gt 0) { $Seconds + 300 } else { 3600 }))
    $runDir = $null
    $lockStarted = $false
    while ((Get-Date) -lt $deadline) {
        $runDir = Get-ChildItem -LiteralPath results -Directory -ErrorAction SilentlyContinue | Where-Object { $_.Name -match '^\d{8}T\d{6}\.\d{9}Z$' -and $_.Name -notin $before } | Sort-Object Name -Descending | Select-Object -First 1
        if ($runDir -and (Test-Path -LiteralPath (Join-Path $runDir.FullName 'summary.json'))) { break }
        if ($Scenario -eq 'db-lock' -and !$lockStarted) {
            $encodedQuery = [Uri]::EscapeDataString('bench_phase{phase="measure"}')
            $state = Invoke-RestMethod "$( $endpoints.prometheus )/api/v1/query?query=$encodedQuery"
            if (@($state.data.result | Where-Object { $_.value[1] -eq '1' }).Count -gt 0) {
                $lockJob = Start-Job -ArgumentList $projectRoot,$endpoints.database_user,$endpoints.database -ScriptBlock {
                    param($root,$dbUser,$dbName)

                    Set-Location $root
                    docker compose exec -T postgres psql -U $dbUser -d $dbName -v ON_ERROR_STOP=1 -c 'BEGIN; LOCK TABLE documents IN SHARE MODE; SELECT pg_sleep(20); ROLLBACK;'
                    if ($LASTEXITCODE -ne 0) { throw 'Database lock injection failed' }
                }
                $lockStarted = $true
            }
        }
        $state = & docker inspect --format '{{.State.Status}}' $containerID
        if ($state -eq 'exited') { throw 'Load generator exited before summary; inspect docker compose logs load' }
        Start-Sleep -Seconds 2
    }
    if (!$runDir -or !(Test-Path -LiteralPath (Join-Path $runDir.FullName 'summary.json'))) { throw 'Benchmark did not finish before deadline' }
    if ($Scenario -eq 'db-lock' -and !$lockStarted) { throw 'Database lock scenario did not inject a lock' }
    if ($lockJob) { $lockJob | Wait-Job | Receive-Job -ErrorAction Stop }
    $path = $runDir.FullName
    $effective = Get-Content -Raw (Join-Path $path 'config.json') | ConvertFrom-Json
    if (!$Scenario) { $Scenario = $effective.scenario }
    $summary = Get-Content -Raw (Join-Path $path 'summary.json') | ConvertFrom-Json
    $start = ([DateTimeOffset]::Parse($summary.MeasurementStarted)).ToUnixTimeSeconds()
    $end = ([DateTimeOffset]::Parse($summary.MeasurementFinished)).ToUnixTimeSeconds()
    $revision = & git rev-parse HEAD
    $dirty = & git status --porcelain
    $dockerInfo = & docker info --format '{{json .}}' | ConvertFrom-Json
    @{ revision=$revision; dirty=[bool]$dirty; scenario=$Scenario; nodes=$Nodes; cpu=$dockerInfo.NCPU; memoryBytes=$dockerInfo.MemTotal; docker=$dockerInfo.ServerVersion; lockSeconds= $(if ($lockStarted) {20} else {0}) } | ConvertTo-Json | Set-Content (Join-Path $path 'environment.json')
    Invoke-StorageCompose config --no-interpolate | Set-Content (Join-Path $path 'compose.yaml')
    Invoke-StorageCompose images --format json | Set-Content (Join-Path $path 'images.json')
    Copy-Item processing-unit/config.yaml (Join-Path $path "processing-unit.yaml")
    Copy-Item router/config.yaml (Join-Path $path "router.yaml")
    $selectors = @('storage_requests_total','storage_request_duration_seconds_bucket','storage_partition_requests_total','storage_pool_connections','storage_pool_acquire_seconds_bucket','storage_db_operation_seconds_bucket','storage_pg_activity_connections','storage_pg_blocking_sessions','storage_pg_statements_calls','storage_pg_statements_execution_seconds','storage_pg_wal_bytes','container_cpu_usage_seconds_total{container_label_com_docker_compose_project="PROJECT_NAME"}','container_memory_working_set_bytes{container_label_com_docker_compose_project="PROJECT_NAME"}','bench_requests_total','bench_dropped_iterations_total','up')
    $snapshots = @{}
    foreach ($selector in $selectors) {
        $selector = $selector.Replace("PROJECT_NAME", $endpoints.project)
        $encodedQuery = [Uri]::EscapeDataString($selector)
        $snapshots[$selector] = Invoke-RestMethod "$( $endpoints.prometheus )/api/v1/query_range?query=$encodedQuery&start=$start&end=$end&step=5s"
    }
    $snapshots | ConvertTo-Json -Depth 30 | Set-Content (Join-Path $path 'prometheus.json')
    $url = "$( $endpoints.grafana )/d/storage-baseline?from=$($start * 1000)&to=$($end * 1000)"
    $url | Set-Content (Join-Path $path 'dashboard-url.txt')
    Write-Output "Results: $path"
    Write-Output "Dashboard: $url"
    Write-Output ($summary | ConvertTo-Json)
} finally {
    if ($containerID) { & docker rm -f $containerID | Out-Null }
    if ($lockJob) { $lockJob | Remove-Job -Force }
}
