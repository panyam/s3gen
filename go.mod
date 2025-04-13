module github.com/panyam/s3gen

go 1.23.5

toolchain go1.24.0

require (
	github.com/adrg/frontmatter v0.2.0
	github.com/felixge/httpsnoop v1.0.4
	github.com/morrisxyang/xreflect v0.0.0-20231001053442-6df0df9858ba
	github.com/panyam/goutils v0.1.3
	github.com/panyam/templar v0.0.15
	github.com/radovskyb/watcher v1.0.7
	github.com/yuin/goldmark v1.7.8
	github.com/yuin/goldmark-highlighting v0.0.0-20220208100518-594be1970594
	go.abhg.dev/goldmark/anchor v0.2.0
)

require (
	github.com/BurntSushi/toml v0.3.1 // indirect
	github.com/alecthomas/chroma v0.10.0 // indirect
	github.com/dlclark/regexp2 v1.11.4 // indirect
	gopkg.in/yaml.v2 v2.3.0 // indirect
)

// replace github.com/panyam/templar v0.0.7 => ../templar/
