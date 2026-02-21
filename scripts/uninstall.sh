BINARY=${BINARY:-miso}
GOBIN=$(go env GOBIN)
[ -z "$GOBIN" ] && GOBIN=$(go env GOPATH)/bin

rm -f $GOBIN/$BINARY
rm -f $GOBIN/misox
