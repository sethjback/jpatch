package jpatch

import (
	"fmt"
	"strconv"
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
	Values map[string]*PathValue
	// Children are the possible paths beneath the current path, based on the Value
	// The key is the the json path attribute, and the returned path segment describes the next
	// level in the path/hierarchy.
	// For Wildcard paths, use "*"
	Children map[string]*PathSegment
}

// PathValue defines an appropriate value for the segment, and what operaitons are permitted on that segment
// SupportedOps is only evaluated when the path is wanting to act on the value itself, i.e. if the path extends to
// children then the child's value and supported ops are used
type PathValue struct {
	Name         string
	SupportedOps []string
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

	vAdd := make([]Patch, 0)
	vRemove := make([]Patch, 0)
	vMove := make([]Patch, 0)
	vReplace := make([]Patch, 0)

	for _, p := range patches {
		err := validatePatch(p)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		finalPath, err := validatePath(p.Path, p.Op, rootSegment)
		if err != nil {
			errs = append(errs, err)
		}
		p.Path = finalPath

		if p.From != "" {
			finalPath, err = validateFrom(p.From, p.Op, rootSegment)
			if err != nil {
				errs = append(errs, err)
			}
			p.From = finalPath
		}
		switch p.Op {
		case Add:
			vAdd = append(vAdd, p)
		case Move:
			vMove = append(vMove, p)
		case Remove:
			vRemove = append(vRemove, p)
		case Replace:
			vReplace = append(vReplace, p)
		}
	}

	vPatches := vRemove
	vPatches = append(vPatches, vReplace...)
	vPatches = append(vPatches, vMove...)
	vPatches = append(vPatches, vAdd...)

	if len(errs) != 0 {
		return nil, errs
	}

	return pable.ValidateJPatchPatches(vPatches)
}

func validatePath(path, op string, root *PathSegment) (string, error) {
	finalPath, lastVal, err := traceObjectPathString(path, op, root)
	if err != nil {
		return finalPath, err
	}

	// Check for valid operations on "-"
	if lastVal.Name == "-" && (op == Remove || op == Test || op == Replace) {
		return "", jpatcherror.New("Invalid Operation", jpatcherror.ErrorInvalidOperation, "cannot "+op+" array index of '-'", nil)
	}

	if !allowedOperation(op, lastVal.SupportedOps) {
		return "", jpatcherror.New("Invalid operation", jpatcherror.ErrorInvalidOperation, fmt.Sprintf("supported operations are: %v", lastVal.SupportedOps), nil)
	}

	return finalPath, nil
}

func validateFrom(path, op string, root *PathSegment) (string, error) {
	finalPath, _, err := traceObjectPathString(path, op, root)
	if err != nil {
		return finalPath, err
	}

	if finalPath == "-" {
		return "", jpatcherror.New("Invalid Operation", jpatcherror.ErrorInvalidOperation, "cannot "+op+" array index of '-'", nil)
	}

	return finalPath, nil
}

// traceObjectPathString looks at a path string and makes sure it is valid according the root segment provided
func traceObjectPathString(path string, op string, root *PathSegment) (string, *PathValue, error) {
	// get rid of the leading ""
	split := strings.Split(path, "/")[1:]
	splitLength := len(split)

	currentSegment := root
	finalPath := ""
	var lastPath *PathValue

	for i, pathSeg := range split {
		pathValue, nextSeg, err := processSegment(currentSegment, pathSeg)
		if err != nil {
			return "", nil, err
		}

		if pathValue.Name == "-" && i < splitLength-1 {
			return "", nil, jpatcherror.New("Invalid Path", jpatcherror.ErrorInvalidPath, `'-' must be final path segment`, nil)
		}

		if nextSeg != nil && i == splitLength-1 && nextSeg.Optional == false {
			return "", nil, jpatcherror.New("Invalid Path", jpatcherror.ErrorInvalidPath, "required path segment missing", nil)
		}

		if nextSeg == nil && i < splitLength-1 {
			return "", nil, jpatcherror.New("Invalid path", jpatcherror.ErrorInvalidPath, "path reaches undefined segment: "+path, nil)
		}

		currentSegment = nextSeg
		lastPath = pathValue

		finalPath += "/" + pathValue.Name
	}

	return finalPath, lastPath, nil
}

func processSegment(seg *PathSegment, path string) (*PathValue, *PathSegment, error) {
	var nextSeg *PathSegment
	val := &PathValue{}
	if seg.Wildcard {
		val.Name = path
		if seg.Values["*"] != nil {
			val.SupportedOps = seg.Values["*"].SupportedOps
		}
		path = "*"
	} else {
		pv, ok := seg.Values[path]
		if !ok {
			return nil, nil, jpatcherror.New("Invalid path", jpatcherror.ErrorInvalidSegment, "unknown segement: "+path, nil)
		}
		val = pv
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

	split := strings.Split(p.Path, "/")[1:]
	splitLength := len(split)
	if splitLength == 0 || (splitLength == 1 && split[0] == "") {
		return jpatcherror.New("Empty Paths Not Supported", jpatcherror.ErrorInvalidPath, "paths must begin with /", p)
	}

	if p.Op == Copy || p.Op == Move {
		split = strings.Split(p.From, "/")[1:]
		splitLength = len(split)
		if splitLength == 0 || (splitLength == 1 && split[0] == "") {
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

func allowedOperation(op string, ops []string) bool {
	for _, o := range ops {
		if op == o {
			return true
		}
	}
	return false
}
