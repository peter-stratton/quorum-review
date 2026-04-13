//go:build tools

package main

import (
	_ "github.com/spf13/cobra"
	_ "github.com/stretchr/testify/assert"
	_ "golang.org/x/tools/go/packages"
	_ "modernc.org/sqlite"
)
