# Generates Go code from .proto files.
# Requires: tools/protoc (locally unpacked protoc) + protoc-gen-go and
# protoc-gen-go-grpc in $(go env GOPATH)/bin.
$ErrorActionPreference = "Stop"

$root     = Split-Path -Parent $PSScriptRoot
$protoc   = Join-Path $root "tools\protoc\bin\protoc.exe"
$goBin    = Join-Path (& go env GOPATH) "bin"

if (-not (Test-Path $protoc)) {
    throw "protoc not found at $protoc - unpack protoc into tools/protoc/"
}

$remoteDir = Join-Path $root "framework\remote\proto"
& $protoc `
    --plugin=protoc-gen-go="$goBin\protoc-gen-go.exe" `
    --plugin=protoc-gen-go-grpc="$goBin\protoc-gen-go-grpc.exe" `
    --proto_path=$remoteDir `
    --go_out=paths=source_relative:$remoteDir `
    --go-grpc_out=paths=source_relative:$remoteDir `
    (Join-Path $remoteDir "envelope.proto")
Write-Host "Generated in $remoteDir"

$fedSrc = Join-Path $root "federated\proto"
$fedOut = Join-Path $root "federated\protogen"
New-Item -ItemType Directory -Force -Path $fedOut | Out-Null
& $protoc `
    --plugin=protoc-gen-go="$goBin\protoc-gen-go.exe" `
    --proto_path=$fedSrc `
    --go_out=paths=source_relative:$fedOut `
    (Join-Path $fedSrc "federated.proto")
Write-Host "Generated in $fedOut"
