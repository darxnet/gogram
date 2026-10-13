package main

import (
	"bytes"
	"embed"
	"go/format"
	"io/fs"
	"log"
	"strings"
	"text/template"
	"unicode"
)

//go:embed templates/*
var embedFS embed.FS

func parseTemplates() *template.Template {
	subFS, err := fs.Sub(embedFS, "templates")
	if err != nil {
		log.Fatalln(err)
	}

	funcMap := template.FuncMap{
		"toTitle":      toTitle,
		"toType":       toType,
		"toLowerFirst": toLowerFirst,
	}

	tmpl, err := template.New("").Option("missingkey=error").Funcs(funcMap).ParseFS(subFS, "*.gotmpl")
	if err != nil {
		log.Fatalln(err)
	}

	return tmpl
}

func isTitle(b byte) bool {
	return b >= 'A' && b <= 'Z'
}

func toTitle(s string) string {
	switch s {
	case "id":
		return "ID"

	case "url":
		return "URL"
	}

	if strings.HasSuffix(s, "_id") {
		s = s[:len(s)-len("_id")] + "ID"
	}

	if strings.HasSuffix(s, "_ids") {
		s = s[:len(s)-len("_ids")] + "IDs"
	}

	nextTitle := true

	return strings.Map(func(r rune) rune {
		if r == '_' {
			nextTitle = true
			return -1
		}

		if nextTitle {
			nextTitle = false
			return unicode.ToTitle(r)
		}

		return r
	}, s)
}

// typeMapping maps Telegram doc type names onto Go types.
var typeMapping = map[string]string{
	// types

	"Integer":           "int64",
	"String":            "string",
	"Integer or String": "ChatID",
	"Boolean":           "bool",
	"True":              "bool",
	"Float":             "float64",

	// methods

	"InlineKeyboardMarkup or ReplyKeyboardMarkup or ReplyKeyboardRemove or ForceReply": "ReplyMarkup",
	"InputFile or String": "InputFile",
	"Int":                 "int64",
}

func toType(s string, required bool) string {
	arrayDepth := 0

	for strings.HasPrefix(s, prefixArrayOf) {
		arrayDepth++
		s = strings.TrimPrefix(s, prefixArrayOf)
	}

	v, ok := typeMapping[s]
	if ok {
		s = v
	} else if strings.HasPrefix(s, "InputMedia") {
		s = "InputMedia"
	}

	// Optional object fields are pointers; ChatID is a small value type
	// with IsZero, omitted through omitzero instead.
	if isTitle(s[0]) && !required && arrayDepth == 0 && s != "ChatID" {
		s = "*" + s
	}

	if arrayDepth == 0 {
		return s
	}

	return strings.Repeat("[]", arrayDepth) + s
}

func toLowerFirst(s string) string {
	return strings.ToLower(s[:1]) + s[1:]
}

func renderTemplate(tmpl *template.Template, name string, data any) []byte {
	var buffer bytes.Buffer

	if err := tmpl.ExecuteTemplate(&buffer, name, data); err != nil {
		log.Fatalln("failed to execute template: ", err)
	}

	formatted, err := format.Source(buffer.Bytes())
	if err != nil {
		log.Printf("failed to format code %q: %v", name, err)
		return buffer.Bytes()
	}

	return formatted
}
