module github.com/panyam/s3gen

go 1.26.0

toolchain go1.27.4

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/adrg/frontmatter v0.2.0
	github.com/felixge/httpsnoop v1.0.4
	github.com/gorilla/mux v1.8.1
	github.com/morrisxyang/xreflect v0.0.0-20231001053442-6df0df9858ba
	github.com/panyam/goutils v0.1.13
	github.com/panyam/servicekit v0.2.6
	github.com/panyam/templar v0.1.0
	github.com/radovskyb/watcher v1.0.7
	github.com/yuin/goldmark v1.7.17
	github.com/yuin/goldmark-highlighting v0.0.0-20220208100518-594be1970594
	go.abhg.dev/goldmark/anchor v0.2.0
	gopkg.in/yaml.v2 v2.3.0
)

require (
	github.com/alecthomas/chroma v0.10.0 // indirect
	github.com/dlclark/regexp2 v1.11.4 // indirect
	github.com/gorilla/websocket v1.5.4-0.20240701034025-d67f41855da4 // indirect
	github.com/panyam/gocurrent v0.1.1 // indirect
	golang.org/x/net v0.61.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// replace github.com/panyam/templar v0.0.19 => ../templar/
