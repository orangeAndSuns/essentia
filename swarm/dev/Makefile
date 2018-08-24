.PHONY: build cluster test

default: build

build:
	go build -o bin/swarm    github.com/orangeAndSuns/essentia/cmd/swarm
	go build -o bin/gess     github.com/orangeAndSuns/essentia/cmd/gess
	go build -o bin/bootnode github.com/orangeAndSuns/essentia/cmd/bootnode

cluster: build
	scripts/boot-cluster.sh

test:
	go test -v github.com/orangeAndSuns/essentia/swarm/...
