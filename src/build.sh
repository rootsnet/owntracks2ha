export GOROOT=/opt/go
export GOPATH=/app/owntracks2ha
export PATH=$PATH:$GOROOT/bin:$GOPATH/bin

cd /app/owntracks2ha/src; go build -o /app/owntracks2ha/bin/owntracks2ha /app/owntracks2ha/src/main.go