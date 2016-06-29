package jpatch

import (
	"fmt"
	"strings"

	"github.com/sethjback/jpatch/jpatcherror"
)

const (
	// Add action
	Add = "add"
	// Remove action
	Remove = "remove"
	// Replace action
	Replace = "replace"
	// Move action
	Move = "move"
	// Copy action
	Copy = "copy"
	// Test action
	Test = "test"
)

// Patch defines an operation to perform on a particular path within a JSON object
type Patch struct {
	// Op is the operation
	Op string `json:"op"`
	// Path is the RFC 6901 path within the object to modify.
	Path string `json:"path"`
	// Value is the new data to set at the path.
	Value interface{} `json:"value,omitempty"`
	// From is the path to move or copy
	From string `json:"from,omitempty"`
}

// PathSegment defines an appropriate object path segment
// PathSegments are a way for an object to tell jpatch what constitutes a valid path within the object.
// Each segment must contain all the possible branches which can be followed from it
//
// Values represent all the possible valid values for the path segement (unless the segment is a wildcard)
//
// Children maps the possible next step segments, indexed by the Value.
// For wildcard segements, use "*" as the index
type PathSegment struct {
	// Optional signals whether this segment is required
	Optional bool
	// Wildcard signals whether any value should be accepted
	Wildcard bool
	// Values has two purposes:
	// 1. it defines acceptable values for this segement
	// 2. it can be used to map the json object property name to a different name in the system.
	// for example, if the path was userName, but the database field was user_name, you can make that adjustment here:
	// map[string]string{"userName":"user_name"}
	Values map[string]string
	// Children are the possible paths beneath the current path, based on the Value
	// The key is the the json path attribute, and the returned path segment describes the next
	// level in the path/hierarchy.
	// For Wildcard paths, use "*"
	Children map[string]*PathSegment
}

// Patchable is the interface that must be implemented to translate the Patch operations
// to valid operations that can be run against the datastore
// The path is translated into a "." separated document path
type Patchable interface {
	// GetJPatchRootSegment returns the root path segment definition
	// All potential patch operations are validatd against this definition
	GetJPatchRootSegment() *PathSegment
	// TranslateValue gives Patchable a chance to validate and modify the value in any way before passing it to the datastore
	ValidateJPatchPatches([]Patch) ([]Patch, []error)
}

// ProcessPatches process patch objects
func ProcessPatches(patches []Patch, pable Patchable) ([]Patch, []error) {

	var errs []error
	rootSegment := pable.GetJPatchRootSegment()

	for _, p := range patches {
		err := validatePatch(p)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		err = validatePath(p.Path, p.Op, rootSegment)
		if err != nil {
			errs = append(errs, err)
		}

		if p.From != "" {
			err = validateFrom(p.From, p.Op, rootSegment)
			if err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(errs) != 0 {
		return nil, errs
	}

	return pable.ValidateJPatchPatches(patches)
}

func validatePath(path, op string, root *PathSegment) error {
	final, err := traceObjectPathString(path, root)
	if err != nil {
		return err
	}

	// Check for valid operations on "-"
	if final == "-" && (op == Remove || op == Test || op == Replace) {
		return jpatcherror.New("Invalid Operation", jpatcherror.ErrorInvalidOperation, "cannot "+op+" array index of '-'", nil)
	}

	return nil
}

func validateFrom(path, op string, root *PathSegment) error {
	final, err := traceObjectPathString(path, root)
	if err != nil {
		return err
	}

	if final == "-" {
		return jpatcherror.New("Invalid Operation", jpatcherror.ErrorInvalidOperation, "cannot "+op+" array index of '-'", nil)
	}

	return nil
}

// traceObjectPathString looks at a path string and makes sure it is valid according the root segment provided
func traceObjectPathString(path string, root *PathSegment) (string, error) {
	// get rid of the leading ""
	split := strings.Split(path, "/")[1:]
	splitLength := len(split)

	currentSegment := root
	finalPath := ""

	for i, pathSeg := range split {
		pathValue, nextSeg, err := processSegment(currentSegment, pathSeg)
		if err != nil {
			return "", err
		}

		if pathValue == "-" && nextSeg != nil {
			return "", jpatcherror.New("Invalid Path", jpatcherror.ErrorInvalidPath, `'-' must be final path segment`, nil)
		}

		if nextSeg != nil && i == splitLength-1 && nextSeg.Optional == false {
			return "", jpatcherror.New("Invalid Path", jpatcherror.ErrorInvalidPath, "required path segment missing", nil)
		}

		if nextSeg == nil && i < splitLength-1 {
			return "", jpatcherror.New("Invalid path", jpatcherror.ErrorInvalidPath, "path reaches undefined segment: "+path, nil)
		}

		currentSegment = nextSeg
		finalPath = pathValue
	}

	return finalPath, nil
}

func processSegment(seg *PathSegment, path string) (string, *PathSegment, error) {
	var nextSeg *PathSegment
	var val string
	if seg.Wildcard {
		val = path
		path = "*"
	} else {
		st, ok := seg.Values[path]
		if !ok {
			return "", nil, jpatcherror.New("Invalid path", jpatcherror.ErrorInvalidSegment, "unknown segement: "+path, nil)
		}
		val = st
	}

	if seg.Children != nil && seg.Children[path] != nil {
		nextSeg = seg.Children[path]
	}

	return val, nextSeg, nil
}

func requiredFurtherSegements(c map[string]*PathSegment) bool {
	for _, seg := range c {
		if !seg.Optional {
			return true
		}
	}
	return false
}

func validatePatch(p Patch) error {

	if !validOperation(p.Op) {
		return jpatcherror.New("Invalid operation", jpatcherror.ErrorInvalidOperation, fmt.Sprintf("supported operations are: %v, %v, %v, %v, %v and %v", Add, Remove, Replace, Copy, Move, Test), p)
	}

	split := strings.Split(p.Path, "/")
	splitLength := len(split)
	if splitLength < 2 || (split[0] == "" && split[1] == "") {
		return jpatcherror.New("Empty Paths Not Supported", jpatcherror.ErrorInvalidPath, "", p)
	}

	if p.Op == Copy || p.Op == Move {
		split = strings.Split(p.From, "/")
		splitLength = len(split)
		if splitLength < 2 || (split[0] == "" && split[1] == "") {
			return jpatcherror.New("From path required", jpatcherror.ErrorInvalidPath, "copy and move operations require from", p)
		}
	}

	if p.Op == Add || p.Op == Replace || p.Op == Test {
		if p.Value == nil {
			return jpatcherror.New("Value required", jpatcherror.ErrorInvalidValue, "value required for "+p.Op, p)
		}
	}

	return nil
}

func validOperation(op string) bool {
	for _, o := range []string{Add, Remove, Replace, Copy, Move, Test} {
		if op == o {
			return true
		}
	}
	return false
}