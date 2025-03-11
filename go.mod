module github.com/panyam/s3gen

go 1.23.5

toolchain go1.24.0

require (
	github.com/adrg/frontmatter v0.2.0
	github.com/alecthomas/chroma/v2 v2.14.0
	github.com/felixge/httpsnoop v1.0.4
	github.com/gorilla/mux v1.8.1
	github.com/morrisxyang/xreflect v0.0.0-20231001053442-6df0df9858ba
	github.com/panyam/goutils v0.1.2
	github.com/panyam/templar v0.0.1
	github.com/radovskyb/watcher v1.0.7
	github.com/yuin/goldmark v1.7.1
	github.com/yuin/goldmark-highlighting/v2 v2.0.0-20230729083705-37449abec8cc
	go.abhg.dev/goldmark/anchor v0.1.1
)

require (
	github.com/BurntSushi/toml v0.3.1 // indirect
	github.com/dlclark/regexp2 v1.11.0 // indirect
	gopkg.in/yaml.v2 v2.3.0 // indirect
)

// replace github.com/panyam/goutils v0.1.2 => ../golang/goutils/
