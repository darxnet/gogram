package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	docLink  = "https://core.telegram.org/bots/api"
	rootLink = "https://core.telegram.org"
)

type Field struct {
	Name, Desc    string
	Type          string
	IsRequired    bool
	Discriminator string // unique value for subtypes
}

func (field Field) IsChatIDString() bool {
	return toTitle(field.Name) == "ChatID" && toType(field.Type, field.IsRequired) == "string"
}

type Type struct {
	Name, Desc string
	Fields     []Field
	Subtypes   []string
}

type Param = Field

type Method struct {
	Name, Desc string
	Params     []Param
	Result     string
}

func (m Method) Multipart() bool {
	for _, field := range m.Params {
		if field.Type == "InputFile" {
			return true
		}
	}

	return false
}

type Info struct {
	Types   map[string]Type
	Methods map[string]Method
}

func main() {
	log.SetFlags(log.Flags() | log.Lshortfile)

	tmpl := parseTemplates()

	pageRC := retrievePageRC()

	root := parseHTML(pageRC)
	_ = pageRC.Close()

	info := parseInfo(root)
	if len(info.Types)*len(info.Methods) == 0 {
		log.Fatalln("cant find types/methods")
	}

	outputs := []struct {
		path     string
		template string
		data     any
	}{
		{path: "./types.gen.go", template: "types.gen.gotmpl", data: info.Types},
		{path: "./methods.gen.go", template: "methods.gen.gotmpl", data: info.Methods},
		{path: "./context.gen.go", template: "context.gen.gotmpl", data: info},
		{path: "./router.gen.go", template: "router.gen.gotmpl", data: info.Types},
	}

	for _, output := range outputs {
		writeFile(output.path, renderTemplate(tmpl, output.template, output.data))
	}
}

func retrievePageRC() io.ReadCloser {
	const timeout = 5 * time.Second

	if localPath := os.Getenv("GOGRAM_HTML"); localPath != "" {
		f, err := os.Open(filepath.Clean(localPath))
		if err != nil {
			log.Fatalln("failed to open html:", err)
		}
		return f
	}

	client := http.Client{Timeout: timeout}

	resp, err := client.Get(docLink)
	if err != nil {
		log.Fatalln("failed to make request:", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("unexpected status code: %d", resp.StatusCode)
	}

	return resp.Body
}

func writeFile(filePath string, code []byte) {
	if err := os.WriteFile(filePath, code, 0o600); err != nil {
		log.Fatalln("failed to write file:", err)
	}
}
