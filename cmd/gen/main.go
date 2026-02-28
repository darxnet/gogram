package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"
)

const (
	docLink  = "https://core.telegram.org/bots/api"
	rootLink = "https://core.telegram.org"
)

// Field describes a Telegram API field for code generation.
type Field struct {
	Name, Desc    string
	Type          string
	IsRequired    bool
	Discriminator string // unique value for subtypes
}

// AutoFillCode returns generated code used to auto-populate a method field from Context.
func (field Field) AutoFillCode(types map[string]Type) string {
	if field.Name == "chat_id" {
		if toType(field.Type, field.IsRequired) == "string" {
			return "ctx.Chat().Identifier()"
		}

		return "ctx.Chat().ID"
	}

	before, _, found := strings.Cut(field.Name, "_id")
	if !found {
		return ""
	}

	if !slices.ContainsFunc(types["Update"].Fields, func(f Field) bool { return f.Name == before }) {
		if field.Name == "callback_query_id" {
			return types["Update"].Fields[13].Name
		}
		return ""
	}

	if !slices.ContainsFunc(types[toTitle(before)].Fields, func(f Field) bool { return f.Name == "id" }) {
		return ""
	}

	return fmt.Sprintf(`func(ctx *Context) string {
                if ctx.update == nil {
                    return ""
                }

                if ctx.update.%[1]s == nil {
                    return ""
                }

                return ctx.update.%[1]s.ID
            }(ctx)`, toTitle(before))
}

// Type describes a Telegram API type for code generation.
type Type struct {
	Name, Desc string
	Fields     []Field
	Subtypes   []string
}

// Param is a method parameter descriptor.
type Param = Field

// Method describes a Telegram API method for code generation.
type Method struct {
	Name, Desc string
	Params     []Param
	Result     string
}

// Multipart reports whether the generated method should use multipart/form-data.
func (m Method) Multipart() bool {
	for _, field := range m.Params {
		if field.Type == "InputFile" {
			return true
		}
	}

	return false
}

// Info aggregates parsed Telegram API types and methods.
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
		//nolint:gosec // G703: we can use any file for tests
		f, err := os.Open(localPath)
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
