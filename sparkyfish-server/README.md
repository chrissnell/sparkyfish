# sparkyfish
The beginnings of a network speed testing tool.

# Usage
```
go get -t github.com/chrissnell/sparkyfish
$GOPATH/bin/sparkyfish --h    # see available options
$GOPATH/bin/sparkyfish -listenport 7121
curl http://localhost:7121 > /dev/null
```
