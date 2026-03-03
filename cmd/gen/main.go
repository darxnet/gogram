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
	skipList := []string{
		"text",
		"caption",
	}

	if slices.Contains(skipList, field.Name) {
		return ""
	}

	switch field.Name {
	case "chat_id":
		switch toType(field.Type, field.IsRequired) {
		case "string":
			return `func(ctx *Context) string {
			    c := ctx.Chat()
				if c == nil {
				    return ""
				}

				return c.Identifier()
			}(ctx)`

		case "int64":
			return `func(ctx *Context) int64 {
			    c := ctx.Chat()
				if c == nil {
				    return 0
				}

				return c.ID
			}(ctx)`
		}

	case "user_id":
		return `func(ctx *Context) int64 {
		    u := ctx.User()
			if u == nil {
			    return 0
			}

			return u.ID
		}(ctx)`

	case "direct_messages_topic_id":
		return `func(ctx *Context) int64 {
		    m := ctx.Message()
			if m == nil {
			    return 0
			}

			return m.DirectMessagesTopic.TopicID
		}(ctx)`

	case "inline_message_id":
		return `func(ctx *Context) string {
      		if ctx.update.CallbackQuery == nil {
      		    return ""
      		}

      		return ctx.update.CallbackQuery.InlineMessageID
       	}(ctx)`
	}

	// auto-fill if field.Name presents in Message

	if slices.ContainsFunc(types["Message"].Fields, func(f Field) bool {
		return f.Name == field.Name && f.Type == field.Type
	}) {
		switch toType(field.Type, field.IsRequired) {
		case "string":
			return fmt.Sprintf(`func(ctx *Context) string {
    			m := ctx.Message()
    			if m == nil {
    			    return ""
    			}

			return m.%s
		}(ctx)`, toTitle(field.Name))

		case "int64":
			return fmt.Sprintf(`func(ctx *Context) int64 {
    			m := ctx.Message()
    			if m == nil {
    			    return 0
    			}

			return m.%s
		}(ctx)`, toTitle(field.Name))
		}
	}

	// overwise for some_name_id auto set update.some_name.id if presents

	before, _, found := strings.Cut(field.Name, "_id")
	if !found {
		return ""
	}

	if !slices.ContainsFunc(types["Update"].Fields, func(f Field) bool { return f.Name == before }) {
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
		typ := toType(field.Type, true)
		if typ == "InputFile" || typ == "[]InputMedia" || typ == "[]InputPaidMedia" {
			return true
		}

		if field.Name == "media" {
			log.Println(field.Type, typ)
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
		{path: "./types.gen.go", template: "types.gen.gotmpl", data: info},
		{path: "./methods.gen.go", template: "methods.gen.gotmpl", data: info},
		{path: "./context.gen.go", template: "context.gen.gotmpl", data: info},
		{path: "./router.gen.go", template: "router.gen.gotmpl", data: info},
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
