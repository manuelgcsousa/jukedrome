bin := "jukedrome"

[private]
default:
    @just --list

build:
    [ -f config.go ] || cp config.go.example config.go
    go build -o {{bin}} .

run: build
    ./{{bin}}

clean:
    rm -f {{bin}}
