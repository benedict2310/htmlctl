package validator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/benedict2310/htmlctl/pkg/model"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func validateParsedFragment(component model.Component, allowlist map[string]struct{}, requireAnchorID bool, expectedAnchorID string) []ValidationError {
	rootNodes, parseErr := parseFragment(component.HTML)
	if parseErr != nil {
		return []ValidationError{newError(component.Name, "parse-fragment", fmt.Sprintf("parse HTML fragment failed: %v", parseErr))}
	}

	rootElements, hasRootText := rootElements(rootNodes)
	var errs []ValidationError

	if hasRootText {
		errs = append(errs, newError(component.Name, "single-root", "text is not allowed at root level; wrap text in a single allowed root element"))
	}

	if len(rootElements) == 0 {
		errs = append(errs, newError(component.Name, "single-root", "component must contain exactly one root element"))
		return errs
	}
	if len(rootElements) > 1 {
		tags := make([]string, 0, len(rootElements))
		for _, n := range rootElements {
			tags = append(tags, n.Data)
		}
		errs = append(errs, newError(component.Name, "single-root", fmt.Sprintf("component has multiple root elements (%s); keep exactly one root element", strings.Join(tags, ", "))))
		return errs
	}

	root := rootElements[0]
	if _, ok := allowlist[root.Data]; !ok {
		tags := make([]string, 0, len(allowlist))
		for tag := range allowlist {
			tags = append(tags, tag)
		}
		sort.Strings(tags)
		errs = append(errs, newError(component.Name, "root-tag-allowlist", fmt.Sprintf("root tag <%s> is not allowed; use one of: %s", root.Data, strings.Join(tags, ", "))))
	}

	if containsScript(root) {
		errs = append(errs, newError(component.Name, "script-disallow", "<script> tags are not allowed in components; move JavaScript to scripts/site.js"))
	}

	if requireAnchorID {
		id, hasID := getAttribute(root, "id")
		if !hasID {
			errs = append(errs, newError(component.Name, "anchor-id", fmt.Sprintf("root element must include id=%q for anchor navigation", expectedAnchorID)))
		} else if id != expectedAnchorID {
			errs = append(errs, newError(component.Name, "anchor-id", fmt.Sprintf("root id mismatch: expected %q, got %q", expectedAnchorID, id)))
		}
	}

	return errs
}

func parseFragment(fragment string) ([]*html.Node, error) {
	ctx := &html.Node{Type: html.ElementNode, DataAtom: atom.Body, Data: "body"}
	return html.ParseFragment(strings.NewReader(fragment), ctx)
}

func rootElements(nodes []*html.Node) ([]*html.Node, bool) {
	roots := make([]*html.Node, 0)
	hasRootText := false
	for _, n := range nodes {
		switch n.Type {
		case html.ElementNode:
			roots = append(roots, n)
		case html.TextNode:
			if strings.TrimSpace(n.Data) != "" {
				hasRootText = true
			}
		}
	}
	return roots, hasRootText
}

func containsScript(root *html.Node) bool {
	if root == nil {
		return false
	}
	if root.Type == html.ElementNode && root.DataAtom == atom.Script {
		return true
	}
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if containsScript(child) {
			return true
		}
	}
	return false
}

func getAttribute(node *html.Node, key string) (string, bool) {
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, key) {
			return attr.Val, true
		}
	}
	return "", false
}

func newError(component, rule, message string) ValidationError {
	return ValidationError{
		Component: component,
		Rule:      rule,
		Severity:  SeverityError,
		Message:   message,
	}
}
