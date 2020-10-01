A lvmd binary release repository by onokatio.

```
mkdir -p build
GOARCH=arm64 GOOS=linux CGO_ENABLED=0 go build -o build/lvmd -ldflags "-X github.com/topolvm/topolvm.Version=$(TOPOLVM_VERSION)" ./pkg/lvmd-linux-x86_64
GOARCH=amd64 GOOS=linux CGO_ENABLED=0 go build -o build/lvmd -ldflags "-X github.com/topolvm/topolvm.Version=$(TOPOLVM_VERSION)" ./pkg/lvmd-linux-aarch64
```
