module github.com/bored-engineer/git-protocol-v2/examples

go 1.23.3

replace github.com/bored-engineer/git-protocol-v2 => ../

require (
	github.com/bored-engineer/git-pkt-line v0.0.0-20250125231634-c00e39a423a0
	github.com/bored-engineer/git-protocol-v2 v0.0.0-00010101000000-000000000000
	github.com/spf13/pflag v1.0.5
)
