param( $revision )

Write-Output "Installing protobuf/protoc-gen-go@$revision ..."

$GOPATH = (go env GOPATH) | Out-String
$GOPATH = $GOPATH.Trim()

go get -d -u github.com/golang/protobuf/protoc-gen-go
git -C "$GOPATH\src\github.com\golang\protobuf" checkout --quiet $revision
go install github.com/golang/protobuf/protoc-gen-go
