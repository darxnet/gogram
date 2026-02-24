package main

import (
	"log"
	"strings"
	"unicode"

	"golang.org/x/net/html"
)

const prefixArrayOf = "Array of "

var replacerDescription = strings.NewReplacer("\n", "\n// ", ". ", ".\n// ")

func parseInfo(root *html.Node) *Info {
	nodesH4 := findNodes(root, "h4")

	info := new(Info)
	info.Types = make(map[string]Type)
	info.Methods = make(map[string]Method)

	for _, nodeH4 := range nodesH4 {
		name := findText(nodeH4)
		if name == "" || name == "InputFile" || strings.ContainsRune(name, ' ') {
			log.Println("skip h4:", name)
			continue
		}

		paragraphs := findNodesSibling(nodeH4, "p")
		tables := findNodesSibling(nodeH4, "table")
		lists := findNodesSibling(nodeH4, "ul")

		desc := parseDescription(findNodesSibling(nodeH4, "p", "ul")...)

		if !unicode.IsLower(rune(name[0])) { // type
			var v Type

			v.Name = name
			v.Desc = desc
			v.Subtypes = parseSubtypes(lists)
			v.Fields = parseFields(tables)

			info.Types[name] = v
		} else { // method
			var v Method

			v.Name = name
			v.Desc = desc
			v.Params = parseParams(tables)
			v.Result = parseResult(paragraphs)

			info.Methods[name] = v
		}
	}

	return info
}

func parseDescriptionTextLinks(node *html.Node) (text, links []byte) {
	if node.Type == html.TextNode {
		text = append(text, replacerDescription.Replace(node.Data)...)
		return text, nil
	}

	if node.Type == html.ElementNode && node.Data == "br" {
		text = append(text, "\n// "...)
		return text, nil
	}

	if node.Type == html.ElementNode && node.Data == "a" {
		href := findAttr(node, "href")

		s := findText(node)
		if s == "" {
			return nil, nil
		}

		text = append(text, "["...)
		text = append(text, s...)
		text = append(text, "]"...)

		if strings.HasPrefix(href, "#") {
			href = docLink + href
		}

		if strings.HasPrefix(href, "/") {
			href = rootLink + href
		}

		if strings.HasPrefix(href, "https://") {
			links = append(links, "\n// ["...)
			links = append(links, s...)
			links = append(links, "]: "...)
			links = append(links, href...)
		}

		return text, links
	}

	if node.Type == html.ElementNode && node.Data == "li" {
		text = append(text, "\n// "...)
	}

	for c := node.FirstChild; c != nil; c = c.NextSibling {
		_text, _links := parseDescriptionTextLinks(c)
		text = append(text, _text...)
		links = append(links, _links...)
	}

	return text, links
}

func parseDescription(nodes ...*html.Node) string {
	var desc []byte
	var links []byte

	for _, p := range nodes {
		text, link := parseDescriptionTextLinks(p)
		if len(text) == 0 && len(link) == 0 {
			continue
		}

		desc = append(desc, "\n// "...)
		desc = append(desc, text...)
		links = append(links, link...)
	}

	if len(links) != 0 {
		desc = append(desc, "\n// "...)
		desc = append(desc, links...)
	}

	return string(desc)
}

func parseSubtypes(lists []*html.Node) []string {
	var subtypes []string

	for _, ul := range lists {
		for _, li := range findNodes(ul, "li") {
			name := findText(li)
			subtypes = append(subtypes, name)
		}
	}

	return subtypes
}

func parseFields(tables []*html.Node) []Field {
	var fields []Field

	for _, table := range tables {
		for _, tr := range findNodes(table, "tr") {
			td := findNodes(tr, "td")
			if len(td) == 0 {
				continue
			}

			if len(td) != 3 {
				log.Fatal("3 table columns expected")
			}

			var field Field

			field.Name = findText(td[0])
			field.Type = findText(td[1])
			field.Desc = parseDescription(td[2])
			field.IsRequired = !strings.HasPrefix(findText(td[2]), "Optional.")

			if strings.Contains(field.Desc, "attach://<file_attach_name>") {
				field.Type = "InputFile"
			}

			field.Discriminator = parseDiscriminator(field.Desc)

			fields = append(fields, field)
		}
	}

	return fields
}

func parseDiscriminator(desc string) string {
	// must be <>
	// always “<>”

	const (
		openQuote  = "“"
		closeQuote = "”"
	)

	words := strings.Fields(desc)
	if len(words) < 3 {
		return ""
	}

	words = words[len(words)-3:]

	switch {
	case words[0] == "must" && words[1] == "be":
		return words[2]

	case words[1] == "always":
		if s := strings.Trim(words[2], openQuote+closeQuote); s != words[2] {
			return s
		}
	}

	return ""
}

func parseParams(tables []*html.Node) []Param {
	var params []Param

	for _, table := range tables {
		for _, tr := range findNodes(table, "tr") {
			td := findNodes(tr, "td")
			if len(td) == 0 {
				continue
			}

			if len(td) != 4 {
				log.Fatal("4 table columns expected")
			}

			var param Param

			param.Name = findText(td[0])
			param.Type = findText(td[1])
			param.Desc = parseDescription(td[3])
			param.IsRequired = findText(td[2]) == "Yes"

			if strings.Contains(param.Desc, "attach://<file_attach_name>") {
				param.Type = "InputFile"
			}

			params = append(params, param)
		}
	}

	return params
}

func parseResult(paragraphs []*html.Node) string {
	keys := []string{"On success", "Returns"}

	for _, p := range paragraphs {
		for node := p.FirstChild; node != nil; node = node.NextSibling {
			if node.Type != html.TextNode {
				continue
			}

			for _, key := range keys {
				if strings.Contains(node.Data, key) {
					s := findText(node.NextSibling)
					if !isTitle(s[0]) {
						continue
					}

					if strings.HasSuffix(node.Data, prefixArrayOf) {
						return prefixArrayOf + s
					}

					return s
				}
			}
		}
	}

	return ""
}
