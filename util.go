package jpatch

import (
	"strconv"
	"strings"
)

func ValidArrayIndex(in interface{}) bool {
	_, ok := in.(int)
	if ok {
		return true
	}

	st, ok := in.(string)
	if ok {
		if st == "-" {
			return true
		}
		if _, err := strconv.Atoi(st); err == nil {
			return true
		}
	}

	return false
}

// Shift adjusts the path segement. Useful for handing the patch off to a child object for processing
func (p Patch) Shift() Patch {
	nPatch := Patch{Op: p.Op, Value: p.Value}

	split := strings.Split(p.Path, "/")[1:]
	nPatch.Path = "/" + strings.Join(split[1:], "/")
	if p.From != "" {
		split = strings.Split(p.From, "/")[1:]
		nPatch.From = "/" + strings.Join(split[1:], "/")
	}

	return nPatch
}

// Segments returns a slice of the path segments
func (p Patch) Segments() []string {
	return strings.Split(p.Path, "/")[1:]
}

// ArrayIndex returns the index int if the final segement of a path is an index
func (p Patch) ArrayIndex(which string) (int, bool) {
	var split []string
	switch which {
	case "path":
		split = strings.Split(p.Path, "/")[1:]
	case "from":
		split = strings.Split(p.From, "/")[1:]
	default:
		return -1, false
	}

	if i, err := strconv.Atoi(split[len(split)-1]); err == nil {
		if i < 0 {
			return -1, false
		}
		return i, true
	}
	return -1, false
}

func (p *PathSegment) AddValue(pathName, actualName string, supportedOps ...string) {
	if p.Values == nil {
		p.Values = make(map[string]*PathValue)
	}

	p.Values[pathName] = &PathValue{actualName, supportedOps}
}

func (p *PathSegment) AddChild(pathName string, child *PathSegment) {
	if p.Children == nil {
		p.Children = make(map[string]*PathSegment)
	}

	p.Children[pathName] = child
}

func (p Patch) PathIndexIn(which string, length int) bool {
	l, ok := p.ArrayIndex(which)
	if !ok || l > length {
		return false
	}
	return ok && l < length
}
