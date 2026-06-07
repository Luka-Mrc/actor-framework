# Generates Go code from .proto files.
# Requires: tools/protoc (locally unpacked protoc) + protoc-gen-go and
# protoc-gen-go-grpc in $(go env GOPATH)/bin.
$ErrorActionPreference = "Stop"

$root     = Split-Path -Parent $PSScriptRoot
$protoc   = Join-Path $root "tools\protoc\bin\protoc.exe"
$goBin    = Join-Path (& go env GOPATH) "bin"
$protoDir = Join-Path $root "framework\remote\proto"

if (-not (Test-Path $protoc)) {
    throw "protoc not found at $protoc - unpack protoc into tools/protoc/"
}

& $protoc `
    --plugin=protoc-gen-go="$goBin\protoc-gen-go.exe" `
    --plugin=protoc-gen-go-grpc="$goBin\protoc-gen-go-grpc.exe" `
    --proto_path=$protoDir `
    --go_out=paths=source_relative:$protoDir `
    --go-grpc_out=paths=source_relative:$protoDir `
    (Join-Path $protoDir "envelope.proto")

Write-Host "Generated in $protoDir"
