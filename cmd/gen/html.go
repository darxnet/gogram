package main

import (
	"io"
	"log"
	"slices"

	"golang.org/x/net/html"
)

func parseHTML(reader io.Reader) *html.Node {
	node, err := html.Parse(reader)
	if err != nil {
		log.Fatalln("failed to parse html:", err)
	}

	return node
}

func findNodes(root *html.Node, tags ...string) []*html.Node {
	var nodes []*html.Node

	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && slices.Contains(tags, node.Data) {
			nodes = append(nodes, node)
		}

		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)

	return nodes
}

func findNodesSibling(root *html.Node, tags ...string) []*html.Node {
	var nodes []*html.Node

	for node := root.NextSibling; node != nil; node = node.NextSibling {
		if node.Data == root.Data {
			break
		}

		if len(node.Data) == len("h1") && node.Data[0] == 'h' {
			break
		}

		if node.Type == html.ElementNode && slices.Contains(tags, node.Data) {
			nodes = append(nodes, node)
		}
	}

	return nodes
}

func findText(node *html.Node) string {
	var text []byte

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		switch {
		case n.Type == html.TextNode:
			text = append(text, n.Data...)

		case n.Type == html.ElementNode && n.Data == "br":
			text = append(text, '\n')
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(node)

	return string(text)
}

func findAttr(node *html.Node, s string) string {
	for _, attr := range node.Attr {
		if attr.Key == s {
			return attr.Val
		}
	}

	return ""
}
