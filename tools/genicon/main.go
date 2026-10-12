// genicon writes the app icon as PNG; used by build.ps1 for the exe resource.
package main

import (
	"os"

	"tvtray/internal/icon"
)

func main() {
	out := "build/icon.png"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	if err := os.WriteFile(out, icon.PNG(256, true), 0o644); err != nil {
		panic(err)
	}
}
