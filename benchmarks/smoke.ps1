param([switch]$FailureChecks, [string]$Api = 'http://127.0.0.1:8080')
$ErrorActionPreference = 'Stop'
function Invoke-StorageCompose {
    & docker compose @args
    if ($LASTEXITCODE -ne 0) { throw "Docker Compose failed (exit $LASTEXITCODE)" }
}
Set-Location (Split-Path $PSScriptRoot -Parent)

$base = $Api
$key = @{ partition_key='smoke'; id=[Guid]::NewGuid().ToString() }
function Command-Check($operation, $body, [int]$expected) {
    $response = Invoke-WebRequest "$base/$operation" -Method Post -ContentType application/json -Body ($body | ConvertTo-Json -Depth 10) -SkipHttpErrorCheck
    if ([int]$response.StatusCode -ne $expected) { throw "$operation expected $expected, got $($response.StatusCode): $($response.Content)" }
    return $response.Content | ConvertFrom-Json
}
$dbStopped = $false
try {
    $create = $key.Clone(); $create.payload = @{ value=1 }
    $first = Command-Check create $create 201
    $null = Command-Check create $create 409
    $read = Command-Check get $key 200
    if ($read.revision -ne $first.revision -or $read.payload.value -ne 1) { throw 'Create/get mismatch' }
    $update = $key.Clone(); $update.payload = @{ value=2 }; $update.expected_revision = $first.revision
    $second = Command-Check update $update 200
    $null = Command-Check update $update 409
    $delete = $key.Clone(); $delete.expected_revision = $first.revision
    $null = Command-Check delete $delete 409
    $delete.expected_revision = $second.revision
    $null = Command-Check delete $delete 200
    $null = Command-Check get $key 404
    $recreated = Command-Check create $create 201
    if ($recreated.revision -eq $first.revision -or $recreated.revision -eq $second.revision) { throw 'Revision reused after recreate' }
    $null = Command-Check delete $delete 409
    if ($FailureChecks) {
        Invoke-StorageCompose stop postgres
        if ($LASTEXITCODE -ne 0) { throw 'Cannot stop test database' }
        $dbStopped = $true
        $response = Invoke-WebRequest "$base/get" -Method Post -ContentType application/json -Body ($key | ConvertTo-Json) -SkipHttpErrorCheck
        if ([int]$response.StatusCode -notin @(503,504)) { throw 'Database failure was not reported' }
        $response = Invoke-WebRequest "$base/readyz" -SkipHttpErrorCheck
        if ([int]$response.StatusCode -ne 503) { throw 'Router remained ready without database' }
        Invoke-StorageCompose up -d --wait postgres
        if ($LASTEXITCODE -ne 0) { throw 'Cannot restart test database' }
        $dbStopped = $false
        $read = Command-Check get $key 200
        if ($read.revision -ne $recreated.revision) { throw 'Document not preserved across database restart' }
    }
    $delete.expected_revision = $recreated.revision
    $null = Command-Check delete $delete 200
    Write-Output 'PASS: Router CRUD, conflicts, delete/recreate and optional database recovery'
} finally {
    if ($dbStopped) { Invoke-StorageCompose up -d --wait postgres }
}
