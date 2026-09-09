$ErrorActionPreference = "Stop"

$forbidden = @(
    "github.com/revel",
    "go.mongodb.org/mongo-driver",
    "github.com/yangphere/leanote/app/lea"
)

foreach ($mode in @("-deps", "-deps -test")) {
    $arguments = @("list") + ($mode -split " ") + @("./app/info")
    $output = & go @arguments
    if ($LASTEXITCODE -ne 0) {
        throw "go list $mode ./app/info failed with exit code $LASTEXITCODE"
    }
    foreach ($pattern in $forbidden) {
        if ($output -match [regex]::Escape($pattern)) {
            throw "forbidden dependency $pattern found in go list $mode ./app/info"
        }
    }
}

Write-Output "app/info dependency graph is framework- and driver-independent"
