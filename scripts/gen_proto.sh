#!/bin/bash
set -euo pipefail

echo "Generating protobuf code for all modules..."

if ! command -v protoc >/dev/null 2>&1; then
  echo "protoc not found in PATH"
  exit 1
fi

go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

found=0
while IFS= read -r -d '' proto; do
  found=1
  protoc \
    --proto_path=. \
    --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    "$proto"
  echo "Generated for $proto"
done < <(find modules -name "*.proto" -print0)

if [ "$found" -eq 0 ]; then
  echo "No proto files found under modules/"
  exit 0
fi

echo "Proto generation completed. Now running CRUD generator..."
go run ./scripts/crud_generator.go
