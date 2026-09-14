param(
    [ValidateSet('constant','ramp','burst','db-lock')][string]$Scenario = 'constant',
    [ValidateSet('uniform','hot')][string]$Distribution = 'uniform',
    [ValidateRange(1,100000)][int]$Rate = 100,
    [ValidateRange(30,1800)][int]$Seconds = 60,
    [ValidateRange(0,100)][int]$ReadPercent = 80,
    [ValidateSet(1,3)][int]$Nodes = 3
)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
Set-Location $projectRoot
function Docker-Checked {
    & docker @args
    if ($LASTEXITCODE -ne 0) { throw "Docker command failed: $args" }
}
$names = 'SCENARIO','DISTRIBUTION','RATE','DURATION','READ_PERCENT','NODES','POOL_MAX','COMPOSE_PROFILES'
$saved = @{}
foreach ($name in $names) { $saved[$name] = [Environment]::GetEnvironmentVariable($name) }
$lockJob = $null
try {
    $env:SCENARIO = if ($Scenario -eq 'db-lock') { 'constant' } else { $Scenario }
    $env:DISTRIBUTION = $Distribution
    $env:RATE = "$Rate"
    $env:DURATION = "${Seconds}s"
    $env:READ_PERCENT = "$ReadPercent"
    $env:NODES = if ($Nodes -eq 1) { 'pu-1=http://pu-1:8080' } else { 'pu-1=http://pu-1:8080,pu-2=http://pu-2:8080,pu-3=http://pu-3:8080' }
    $env:POOL_MAX = if ($Nodes -eq 1) { '24' } else { '8' }
    $env:COMPOSE_PROFILES = if ($Nodes -eq 1) { 'bench' } else { 'multi,bench' }
    Docker-Checked compose stop load
    if ($Nodes -eq 1) { Docker-Checked compose --profile multi stop pu-2 pu-3 }
    $services = @('postgres','pu-1','router','postgres-exporter','cadvisor','prometheus','grafana')
    if ($Nodes -eq 3) { $services += @('pu-2','pu-3') }
    Docker-Checked compose up -d --wait @services
    $before = @(Get-ChildItem -LiteralPath results -Directory -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Name)
    Docker-Checked compose up -d --force-recreate load
    $deadline = (Get-Date).AddSeconds($Seconds + 300)
    $runDir = $null
    $lockStarted = $false
    while ((Get-Date) -lt $deadline) {
        $runDir = Get-ChildItem -LiteralPath results -Directory -ErrorAction SilentlyContinue | Where-Object { $_.Name -match '^\d{8}T\d{6}\.\d{9}Z$' -and $_.Name -notin $before } | Sort-Object Name -Descending | Select-Object -First 1
        if ($runDir -and (Test-Path -LiteralPath (Join-Path $runDir.FullName 'summary.json'))) { break }
        if ($Scenario -eq 'db-lock' -and !$lockStarted) {
            $q = [Uri]::EscapeDataString('bench_phase{phase="measure"}')
            $state = Invoke-RestMethod "http://localhost:9090/api/v1/query?query=$q"
            if (@($state.data.result | Where-Object { $_.value[1] -eq '1' }).Count -gt 0) {
                $lockJob = Start-Job -ArgumentList $projectRoot -ScriptBlock {
                    param($root)
                    Set-Location $root
                    & docker compose exec -T postgres psql -U storage -d storage -v ON_ERROR_STOP=1 -c 'BEGIN; LOCK TABLE documents IN SHARE MODE; SELECT pg_sleep(20); ROLLBACK;'
                    if ($LASTEXITCODE -ne 0) { throw 'Database lock injection failed' }
                }
                $lockStarted = $true
            }
        }
        $container = & docker compose ps -a --format json load | ConvertFrom-Json
        if ($container.State -eq 'exited') { throw 'Load generator exited before summary; inspect docker compose logs load' }
        Start-Sleep -Seconds 2
    }
    if (!$runDir -or !(Test-Path -LiteralPath (Join-Path $runDir.FullName 'summary.json'))) { throw 'Benchmark did not finish before deadline' }
    if ($Scenario -eq 'db-lock' -and !$lockStarted) { throw 'Database lock scenario did not inject a lock' }
    if ($lockJob) { $lockJob | Wait-Job | Receive-Job -ErrorAction Stop }
    $path = $runDir.FullName
    $summary = Get-Content -Raw (Join-Path $path 'summary.json') | ConvertFrom-Json
    $start = ([DateTimeOffset]::Parse($summary.MeasurementStarted)).ToUnixTimeSeconds()
    $end = ([DateTimeOffset]::Parse($summary.MeasurementFinished)).ToUnixTimeSeconds()
    $revision = & git rev-parse HEAD
    $dirty = & git status --porcelain
    $dockerInfo = & docker info --format '{{json .}}' | ConvertFrom-Json
    @{ revision=$revision; dirty=[bool]$dirty; scenario=$Scenario; nodes=$Nodes; cpu=$dockerInfo.NCPU; memoryBytes=$dockerInfo.MemTotal; docker=$dockerInfo.ServerVersion; lockSeconds= $(if ($lockStarted) {20} else {0}) } | ConvertTo-Json | Set-Content (Join-Path $path 'environment.json')
    & docker compose config | Set-Content (Join-Path $path 'compose.yaml')
    & docker compose images --format json | Set-Content (Join-Path $path 'images.json')
    $selectors = @('storage_requests_total','storage_request_duration_seconds_bucket','storage_partition_requests_total','storage_pool_connections','storage_pool_acquire_seconds_bucket','storage_db_operation_seconds_bucket','storage_pg_activity_connections','storage_pg_blocking_sessions','storage_pg_statements_calls','storage_pg_statements_execution_seconds','storage_pg_wal_bytes','container_cpu_usage_seconds_total{container_label_com_docker_compose_project="autoscale-storage"}','container_memory_working_set_bytes{container_label_com_docker_compose_project="autoscale-storage"}','bench_requests_total','bench_dropped_iterations_total','up')
    $snapshots = @{}
    foreach ($selector in $selectors) {
        $q = [Uri]::EscapeDataString($selector)
        $snapshots[$selector] = Invoke-RestMethod "http://localhost:9090/api/v1/query_range?query=$q&start=$start&end=$end&step=5s"
    }
    $snapshots | ConvertTo-Json -Depth 30 | Set-Content (Join-Path $path 'prometheus.json')
    $url = "http://localhost:3000/d/storage-baseline?from=$($start * 1000)&to=$($end * 1000)"
    $url | Set-Content (Join-Path $path 'dashboard-url.txt')
    Write-Output "Results: $path"
    Write-Output "Dashboard: $url"
    Write-Output ($summary | ConvertTo-Json)
} finally {
    if ($lockJob) { $lockJob | Remove-Job -Force }
    foreach ($name in $names) { [Environment]::SetEnvironmentVariable($name, $saved[$name]) }
}
