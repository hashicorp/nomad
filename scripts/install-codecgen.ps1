param( $revision )

Write-Output "Installing codec/codec-gen@$revision ..."

$GOPATH = (go env GOPATH) | Out-String
$GOPATH = $GOPATH.Trim()

go get -d -u github.com/ugorji/go/codec/codecgen
git -C "$GOPATH\src\github.com\ugorji\go\codec" checkout --quiet $revision
go install github.com/ugorji/go/codec/codecgen
