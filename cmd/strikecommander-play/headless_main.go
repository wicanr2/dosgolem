//go:build headless

package main

import "os"

func main() { o := parse(); o.headless = true; os.Exit(runHeadless(o)) }
