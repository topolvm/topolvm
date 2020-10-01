A lvmd binary release repository by onokatio.

```
mkdir -p build
TOPOLVM_VERSION=devel
GOARCH=amd64 GOOS=linux CGO_ENABLED=0 go build -o build/lvmd-linux-x86_64 -ldflags "-X github.com/topolvm/topolvm.Version=$TOPOLVM_VERSION" ./pkg/lvmd
GOARCH=arm64 GOOS=linux CGO_ENABLED=0 go build -o build/lvmd-linux-aarch -ldflags "-X github.com/topolvm/topolvm.Version=$TOPOLVM_VERSION" ./pkg/lvmd
```
