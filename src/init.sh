export GOROOT=/opt/go
export GOPATH=/app/owntracks2ha
export PATH=$PATH:$GOROOT/bin:$GOPATH/bin

cd /app/owntracks2ha/src/
go mod init owntracks2ha 
go get gopkg.in/yaml.v2
go get github.com/eclipse/paho.mqtt.golang