param(
    [ValidateSet('Apply', 'Status')]
    [string]$Action = 'Apply',
    [switch]$SkipBuild
)

$ErrorActionPreference = 'Stop'
$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$namespace = 'autoscale-storage'
$operatorContext = 'autoscale-storage'
$routerPublicHost = 'api.autoscale-storage.localhost'

function Invoke-K3sKubectl {
    param(
        [Parameter(Mandatory)]
        [string]$Arguments,
        [string]$InputObject
    )

    $command = "k3s-kubectl $Arguments"
    if ($PSBoundParameters.ContainsKey('InputObject')) {
        $InputObject | wsl.exe -- bash -ic $command
    } else {
        wsl.exe -- bash -ic $command
    }
    if ($LASTEXITCODE -ne 0) {
        throw "k3s-kubectl failed with exit code $LASTEXITCODE"
    }
}

function ConvertTo-WslPath {
    param(
        [Parameter(Mandatory)]
        [string]$Path
    )

    $wslPath = (wsl.exe -- wslpath -a ($Path -replace '\\', '/')).Trim()
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($wslPath)) {
        throw "Failed to translate path for WSL: $Path"
    }
    return $wslPath
}

function Invoke-K3sHelm {
    param(
        [Parameter(Mandatory)]
        [string]$Arguments
    )

    wsl.exe -- bash -lc "helm --kubeconfig '$script:K3sKubeconfig' $Arguments"
    if ($LASTEXITCODE -ne 0) {
        throw "Helm failed with exit code $LASTEXITCODE"
    }
}

function Get-DotEnvValue {
    param(
        [Parameter(Mandatory)]
        [string[]]$Lines,
        [Parameter(Mandatory)]
        [string]$Name
    )

    $prefix = "$Name="
    $line = $Lines | Where-Object { $_.TrimStart().StartsWith($prefix) } | Select-Object -Last 1
    if (-not $line) {
        throw "$Name is missing from .env"
    }
    $value = $line.Trim().Substring($prefix.Length)
    if (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'"))) {
        $value = $value.Substring(1, $value.Length - 2)
    }
    if ([string]::IsNullOrWhiteSpace($value)) {
        throw "$Name is empty in .env"
    }
    return $value
}

function Set-OperatorContext {
    $cluster = (wsl.exe -- bash -ic "k3s-kubectl config view --minify -o jsonpath='{.contexts[0].context.cluster}'").Trim()
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($cluster)) {
        throw 'Failed to read the k3s cluster name'
    }
    $token = (wsl.exe -- bash -ic "k3s-kubectl --namespace autoscale-storage create token operator --duration=24h").Trim()
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($token)) {
        throw 'Failed to create the short-lived operator token'
    }
    Invoke-K3sKubectl "config set-credentials autoscale-storage-operator --token='$token'"
    Invoke-K3sKubectl "config set-context autoscale-storage --cluster='$cluster' --user=autoscale-storage-operator --namespace=autoscale-storage"
    Invoke-K3sKubectl "--context autoscale-storage auth can-i get pods"
    $clusterAccess = (wsl.exe -- bash -ic "k3s-kubectl --context autoscale-storage auth can-i get nodes").Trim()
    if ($clusterAccess -ne 'no') {
        throw 'The operator context unexpectedly has cluster node access'
    }
}

function Import-ApplicationImages {
    $images = @(
        @{ Name = 'autoscale-processing-unit:local'; Context = 'processing-unit' },
        @{ Name = 'autoscale-router:local'; Context = 'router' },
        @{ Name = 'autoscale-control-plane:local'; Context = 'control-plane' },
        @{ Name = 'autoscale-load:local'; Context = 'benchmarks/load' }
    )
    foreach ($image in $images) {
        docker build --tag $image.Name (Join-Path $repositoryRoot $image.Context)
        if ($LASTEXITCODE -ne 0) {
            throw "Docker build failed for $($image.Name)"
        }
    }

    $resultsDirectory = Join-Path $repositoryRoot 'results/kubernetes'
    New-Item -ItemType Directory -Force -Path $resultsDirectory | Out-Null
    $archivePath = Join-Path $resultsDirectory 'application-images.tar'
    try {
        docker save --output $archivePath ($images.Name)
        if ($LASTEXITCODE -ne 0) {
            throw 'Docker image export failed'
        }
        $wslArchivePath = (wsl.exe -- wslpath -a ($archivePath -replace '\\', '/')).Trim()
        wsl.exe -u root -- k3s ctr images import $wslArchivePath
        if ($LASTEXITCODE -ne 0) {
            throw 'k3s image import failed'
        }
    } finally {
        if (Test-Path $archivePath) {
            Remove-Item -LiteralPath $archivePath
        }
    }
}

function Set-LocalRouterTlsSecret {
    $tlsDirectory = Join-Path $repositoryRoot 'results/kubernetes/tls'
    New-Item -ItemType Directory -Force -Path $tlsDirectory | Out-Null
    $certificatePath = Join-Path $tlsDirectory 'router.crt'
    $privateKeyPath = Join-Path $tlsDirectory 'router.key'

    if (-not (Test-Path $certificatePath) -or -not (Test-Path $privateKeyPath)) {
        wsl.exe -- openssl version | Out-Null
        if ($LASTEXITCODE -ne 0) {
            throw 'openssl is required in WSL to generate the local Router TLS certificate'
        }
        $wslCertificatePath = ConvertTo-WslPath $certificatePath
        $wslPrivateKeyPath = ConvertTo-WslPath $privateKeyPath
        wsl.exe -- openssl req -x509 -nodes -newkey rsa:2048 -sha256 -days 365 `
            -keyout $wslPrivateKeyPath `
            -out $wslCertificatePath `
            -subj "/CN=$routerPublicHost" `
            -addext "subjectAltName=DNS:$routerPublicHost"
        if ($LASTEXITCODE -ne 0) {
            throw 'Failed to generate the local Router TLS certificate'
        }
    }

    $tlsSecretManifest = @{
        apiVersion = 'v1'
        kind = 'Secret'
        metadata = @{ name = 'router-tls'; namespace = $namespace }
        type = 'kubernetes.io/tls'
        data = @{
            'tls.crt' = [Convert]::ToBase64String([IO.File]::ReadAllBytes($certificatePath))
            'tls.key' = [Convert]::ToBase64String([IO.File]::ReadAllBytes($privateKeyPath))
        }
    } | ConvertTo-Json -Depth 5
    Invoke-K3sKubectl 'apply -f -' -InputObject $tlsSecretManifest
}

function Remove-LegacyPrometheusRulesVolume {
    $statefulSetJson = wsl.exe -- bash -ic "k3s-kubectl --namespace $namespace get statefulset prometheus -o json"
    if ($LASTEXITCODE -ne 0) {
        throw 'Failed to inspect the Prometheus StatefulSet after Helm adoption'
    }
    $statefulSet = $statefulSetJson | ConvertFrom-Json
    $containerNames = @($statefulSet.spec.template.spec.containers | ForEach-Object { $_.name })
    $prometheusContainerIndex = [Array]::IndexOf($containerNames, 'prometheus')
    if ($prometheusContainerIndex -lt 0) {
        throw 'Prometheus container is missing from its StatefulSet'
    }
    $prometheusContainer = $statefulSet.spec.template.spec.containers[$prometheusContainerIndex]

    $volumeMountNames = @($prometheusContainer.volumeMounts | ForEach-Object { $_.name })
    $volumeNames = @($statefulSet.spec.template.spec.volumes | ForEach-Object { $_.name })
    $rulesMountIndex = [Array]::IndexOf($volumeMountNames, 'rules')
    $rulesVolumeIndex = [Array]::IndexOf($volumeNames, 'rules')
    if ($rulesMountIndex -lt 0 -and $rulesVolumeIndex -lt 0) {
        return
    }
    if ($rulesMountIndex -lt 0 -or $rulesVolumeIndex -lt 0) {
        throw 'Legacy Prometheus rules volume is only partially present; refusing an ambiguous migration'
    }

    $jsonPatch = @(
        @{ op = 'remove'; path = "/spec/template/spec/containers/$prometheusContainerIndex/volumeMounts/$rulesMountIndex" },
        @{ op = 'remove'; path = "/spec/template/spec/volumes/$rulesVolumeIndex" }
    ) | ConvertTo-Json -Compress
    wsl.exe -- env "KUBECONFIG=$script:K3sKubeconfig" kubectl --namespace $namespace patch statefulset prometheus --type=json --patch $jsonPatch
    if ($LASTEXITCODE -ne 0) {
        throw 'Failed to remove the legacy Prometheus rules volume after Helm adoption'
    }
}

function Remove-LegacyKustomizeConfigMaps {
    $configMapNames = wsl.exe -- env "KUBECONFIG=$script:K3sKubeconfig" kubectl --namespace $namespace get configmaps -o 'jsonpath={range .items[*]}{.metadata.name}{"\n"}{end}'
    if ($LASTEXITCODE -ne 0) {
        throw 'Failed to inspect ConfigMaps after Helm adoption'
    }
    $legacyNamePattern = '^(control-plane-config|grafana-dashboards|grafana-dashboards-provider|grafana-datasource|load-config|postgres-exporter-config|postgres-init|processing-unit-config|prometheus-config|prometheus-rules|router-config)-[a-z0-9]{10}$'
    foreach ($configMapName in @($configMapNames -split "`r?`n" | Where-Object { $_ -match $legacyNamePattern })) {
        Invoke-K3sKubectl "--namespace $namespace delete configmap '$configMapName'"
    }
}

if ($Action -eq 'Status') {
    Set-OperatorContext
    Invoke-K3sKubectl "--context $operatorContext get pods,services,ingresses,persistentvolumeclaims,jobs -o wide"
    exit 0
}

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    throw 'docker is required to build the local application images'
}
$helmPath = (wsl.exe -- bash -lc 'command -v helm').Trim()
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($helmPath)) {
    throw 'helm is required in WSL to deploy the local chart'
}
$kubectlAlias = (wsl.exe -- bash -ic 'alias k3s-kubectl').Trim()
if ($LASTEXITCODE -ne 0 -or $kubectlAlias -notmatch 'KUBECONFIG=([^ ]+)') {
    throw 'k3s-kubectl alias must contain an explicit KUBECONFIG path'
}
$script:K3sKubeconfig = $Matches[1].Trim("'")

$environmentPath = Join-Path $repositoryRoot '.env'
if (-not (Test-Path $environmentPath)) {
    throw '.env is missing; copy .env.example once and set local passwords'
}
$environmentLines = Get-Content $environmentPath
$postgresPassword = Get-DotEnvValue -Lines $environmentLines -Name 'POSTGRES_PASSWORD'
$monitorPassword = Get-DotEnvValue -Lines $environmentLines -Name 'MONITOR_PASSWORD'

if (-not $SkipBuild) {
    Import-ApplicationImages
}

$namespaceManifest = Get-Content (Join-Path $PSScriptRoot 'foundation/namespace.yaml') -Raw
Invoke-K3sKubectl 'apply -f -' -InputObject $namespaceManifest

$secretManifest = @{
    apiVersion = 'v1'
    kind = 'Secret'
    metadata = @{ name = 'storage-credentials'; namespace = $namespace }
    type = 'Opaque'
    stringData = @{
        POSTGRES_PASSWORD = $postgresPassword
        MONITOR_PASSWORD = $monitorPassword
    }
} | ConvertTo-Json -Depth 5
Invoke-K3sKubectl 'apply -f -' -InputObject $secretManifest
Set-LocalRouterTlsSecret
Invoke-K3sKubectl 'delete clusterrolebinding autoscale-storage-kube-state-metrics --ignore-not-found'
Invoke-K3sKubectl 'delete clusterrole autoscale-storage-kube-state-metrics --ignore-not-found'

$chartPath = ConvertTo-WslPath $PSScriptRoot
$helmArguments = @(
    'upgrade', '--install', 'autoscale-storage', "'$chartPath'",
    '--namespace', $namespace,
    '--values', "'$chartPath/values-k3s.yaml'",
    '--atomic', '--wait', '--wait-for-jobs', '--timeout', '5m', '--take-ownership'
)
$configFiles = [ordered]@{
    'configs.processingUnit' = (Join-Path $repositoryRoot 'processing-unit/config.yaml')
    'configs.router' = (Join-Path $repositoryRoot 'router/config.yaml')
    'configs.controlPlane' = (Join-Path $repositoryRoot 'control-plane/config.yaml')
    'configs.load' = (Join-Path $PSScriptRoot 'load/config.yaml')
    'configs.prometheus' = (Join-Path $PSScriptRoot 'prometheus/config.yaml')
    'configs.postgresExporter' = (Join-Path $repositoryRoot 'deploy/postgres-exporter/queries.yaml')
    'configs.migrationDocuments' = (Join-Path $repositoryRoot 'processing-unit/migrations/001_documents.sql')
    'configs.migrationMonitoring' = (Join-Path $repositoryRoot 'processing-unit/migrations/002_monitoring.sql')
    'configs.migrationAssignment' = (Join-Path $repositoryRoot 'processing-unit/migrations/003_assignment.sql')
    'configs.grafanaDatasource' = (Join-Path $repositoryRoot 'deploy/grafana/provisioning/datasources/prometheus.yaml')
    'configs.grafanaDashboardProvider' = (Join-Path $repositoryRoot 'deploy/grafana/provisioning/dashboards/default.yml')
    'configs.grafanaStorageDashboard' = (Join-Path $repositoryRoot 'deploy/grafana/dashboards/storage.json')
    'configs.grafanaControlPlaneDashboard' = (Join-Path $repositoryRoot 'deploy/grafana/dashboards/control-plane.json')
}
foreach ($configEntry in $configFiles.GetEnumerator()) {
    $configPath = ConvertTo-WslPath $configEntry.Value
    $helmArguments += @('--set-file', "$($configEntry.Key)='$configPath'")
}
Invoke-K3sHelm ($helmArguments -join ' ')
Remove-LegacyPrometheusRulesVolume
Set-OperatorContext
Invoke-K3sKubectl "--context $operatorContext rollout restart deployment/control-plane deployment/router statefulset/processing-unit"

Invoke-K3sKubectl "--context $operatorContext rollout status statefulset/postgres --timeout=5m"
Invoke-K3sKubectl "--context $operatorContext rollout status statefulset/processing-unit --timeout=5m"
Invoke-K3sKubectl "--context $operatorContext rollout status deployment/control-plane --timeout=5m"
Invoke-K3sKubectl "--context $operatorContext wait --for=condition=complete job/assignment-bootstrap --timeout=5m"
Invoke-K3sKubectl "--context $operatorContext rollout status deployment/router --timeout=5m"
Invoke-K3sKubectl "--context $operatorContext rollout status statefulset/prometheus --timeout=5m"
Invoke-K3sKubectl "--context $operatorContext rollout status deployment/grafana --timeout=5m"
Invoke-K3sKubectl "--context $operatorContext get pods -o wide"
Remove-LegacyKustomizeConfigMaps
