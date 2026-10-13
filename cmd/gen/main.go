package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
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
	Alternative   string // "required if <Alternative> isn't specified": one of the two must be set
}

//nolint:gocritic // hugeParam: templates call methods on range values, which need value receivers
func (field Field) FileUploadCode(types map[string]Type) string {
	buffer := new(bytes.Buffer)

	fieldTitle := toTitle(field.Name)
	fieldType := toType(field.Type, true)
	fieldExpr := "params." + fieldTitle

	if fieldType == "InputFile" {
		inputFileUploadCode(buffer, fieldExpr, fmt.Sprintf("%q", field.Name), field.IsRequired)

		if buffer.Len() != 0 {
			return buffer.String()
		}
	}

	var isArray bool
	if _, after, found := strings.Cut(fieldType, "[]"); found {
		fieldType = after
		isArray = true
	}

	parent, ok := types[fieldType]
	if !ok {
		return ""
	}

	if isArray {
		_, _ = fmt.Fprintf(buffer, "for i := range %s {\n", fieldExpr)
		if inputFileFieldsCode(buffer, parent.Fields, fieldExpr+"[i]", field.Name, true) {
			buffer.WriteString("}\n")
			return buffer.String()
		}

		buffer.Reset()

		_, _ = fmt.Fprintf(buffer, "for i := range %s {\n", fieldExpr)
		if inputFileSubtypesCode(buffer, parent.Subtypes, fieldExpr+"[i]", field.Name, true, types) {
			buffer.WriteString("}\n")
			return buffer.String()
		}

		buffer.Reset()
	}

	if inputFileFieldsCode(buffer, parent.Fields, fieldExpr, field.Name, false) {
		return buffer.String()
	}

	buffer.Reset()

	if inputFileSubtypesCode(buffer, parent.Subtypes, fieldExpr, field.Name, false, types) {
		return buffer.String()
	}

	return ""
}

// AutoFillCode returns generated code used to auto-populate a method field from Context.
//
//nolint:gocritic // hugeParam: templates call methods on range values, which need value receivers
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
		case "ChatID":
			return "ctx.chatTarget()"

		case "int64":
			return "ctx.chatID()"
		}

	case "user_id":
		return "ctx.userID()"

	case "direct_messages_topic_id":
		return "ctx.directMessagesTopicID()"

	case "inline_message_id":
		return "ctx.inlineMessageID()"
	}

	// auto-fill if field.Name presents in Message

	if slices.ContainsFunc(types["Message"].Fields, func(f Field) bool {
		return f.Name == field.Name && f.Type == field.Type
	}) {
		typ := toType(field.Type, field.IsRequired)
		if typ == "string" || typ == "int64" {
			return fmt.Sprintf("messageField(ctx, func(m *Message) %s { return m.%s })", typ, toTitle(field.Name))
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

	return fmt.Sprintf(`updateField(ctx, func(u *Update) string {
		if u.%[1]s == nil {
			return ""
		}

		return u.%[1]s.ID
	})`, toTitle(before))
}

// Type describes a Telegram API type for code generation.
type Type struct {
	Name, Desc  string
	Fields      []Field
	Subtypes    []string
	CanBeString bool   // value can be represented as a plain string
	CanBeArray  string // element type name when value can be a JSON array (e.g. "RichText")
}

// Param is a method parameter descriptor.
type Param = Field

// Method describes a Telegram API method for code generation.
type Method struct {
	Name, Desc string
	Params     []Param
	Result     string
}

// Variant is one generated Context method of a Method: Suffix on the name
// and the parameters passed positionally. The main variant (empty Suffix)
// takes the required parameters plus the first field of every
// "required if the other isn't specified" pair; each further variant swaps
// in the other field of a pair, e.g. EditMessageTextWithRichMessage.
type Variant struct {
	Suffix string
	Params []Param
}

// Variants lists the Context method variants of m.
func (m Method) Variants(types map[string]Type) []Variant {
	positional := func(include func(i int, p Param) bool) []Param {
		var out []Param

		for i, p := range m.Params {
			if p.AutoFillCode(types) != "" {
				continue
			}

			if p.IsRequired || include(i, p) {
				out = append(out, p)
			}
		}

		return out
	}

	indexOf := func(name string) int {
		return slices.IndexFunc(m.Params, func(p Param) bool { return p.Name == name })
	}

	// A pair is ordered as in the docs: the first field is the main variant.
	isFirst := func(i int, p Param) bool {
		return p.Alternative != "" && indexOf(p.Alternative) > i
	}

	variants := []Variant{{Params: positional(isFirst)}}

	for i, p := range m.Params {
		if p.Alternative == "" || isFirst(i, p) || indexOf(p.Alternative) == -1 {
			continue
		}

		variants = append(variants, Variant{
			Suffix: "With" + toTitle(p.Name),
			Params: positional(func(j int, _ Param) bool { return j == i }),
		})
	}

	return variants
}

// Multipart reports whether the generated method should use multipart/form-data.
func (m Method) Multipart(types map[string]Type) bool {
	for _, field := range m.Params {
		if field.FileUploadCode(types) != "" {
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

	outDir := flag.String("out", ".", "directory to write the generated files into")
	flag.Parse()

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
		{path: "types.gen.go", template: "types.gen.gotmpl", data: info},
		{path: "methods.gen.go", template: "methods.gen.gotmpl", data: info},
		{path: "context.gen.go", template: "context.gen.gotmpl", data: info},
		{path: "router.gen.go", template: "router.gen.gotmpl", data: info},
	}

	for _, output := range outputs {
		writeFile(filepath.Join(*outDir, output.path), renderTemplate(tmpl, output.template, output.data))
	}
}

func retrievePageRC() io.ReadCloser {
	const timeout = 30 * time.Second

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
	//nolint:gosec // G306: generated source, not a secret
	if err := os.WriteFile(filePath, code, 0o644); err != nil {
		log.Fatalln("failed to write file:", err)
	}
}
