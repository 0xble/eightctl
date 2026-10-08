// Command eightsleep controls an Eight Sleep Pod. Its operations are declared
// in ops. The release also ships it as eightctl, the name it had before.
package main

import (
	"github.com/0xble/toolkit"

	"github.com/0xble/eightsleep/ops"
)

var version = "dev"

func main() {
	b := ops.NewBackend(version)
	toolkit.Main(ops.New(version, b), ops.Options(b))
}
