package schemas

import "embed"

//go:embed *.schema.json
var files embed.FS

// Read returns a packaged JSON Schema by its file name.
func Read(name string) ([]byte, error) {
	return files.ReadFile(name)
}
