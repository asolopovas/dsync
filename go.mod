module github.com/asolopovas/dsync

go 1.20

retract (
	v1.0.0 // Published accidentally.
	v0.1.0 // Published accidentally.
)

require github.com/spf13/cobra v1.8.0

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
)
