package jpatch

import (
	"strconv"
	"strings"

	"gitlab.com/delvecore/api/validator"
	"gitlab.com/paasapi/api/apierr"
	"gitlab.com/paasapi/api/db"
	"gitlab.com/paasapi/api/util"
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
}

// PathSegment defines an appropriate object path segment
// PathSegments are a way for an object to tell jpatch what constitutes a valie path within the object.
// Each segment must contain all the possible branches which can be followed from it
//
// Values represent all the possible valid values for the path segement (unless the segment is a wildcard)
//
// Children maps the possible next step segments, indexed by the Value.
// For wildcard segements, use "*" as the index
type PathSegment struct {
	// Optional signals whether this segment is required
	Optional bool
	// Wildcard signals wether any value should be accepted
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
	ValidateJPatchPatches([]Patch) ([]Patch, error)
}

// TranslatePatches takes a slice of patches and generates the update expression using the patchable passed in
func TranslatePatches(patches []Patch, pable Patchable) (map[string]db.UpdateValue, apierr.Error) {
	valMap := map[string]db.UpdateValue{}
	var errs []apierr.Error
	for _, p := range patches {
		if p.Op != Add && p.Op != Remove && p.Op != Replace {
			return nil, apierr.New("Invalid operation", ErrorInvalidOperation, nil, "Supported operations are \"add\" and \"remove\"")
		}
		path, err := translatePath(p, pable)
		if err != nil {
			return nil, err
		}

		val, err := pable.ValidatePatchValue(path, p.Op, p.Value)
		if err != nil {
			errs = append(errs, err)
		} else {
			uv := db.UpdateValue{Value: val}

			switch p.Op {
			case Add:
				uv.Action = db.Put
			case Replace:
				uv.Action = db.Update
			case Remove:
				uv.Action = db.Delete
			}

			valMap[path] = uv
		}
	}

	if len(errs) != 0 {
		return nil, apierr.New("Patch value invalid", validator.ErrorValidation, nil, errs)
	}
	return valMap, nil
}

func translatePath(p Patch, patch Patchable) (string, apierr.Error) {
	split := strings.Split(p.Path, "/")
	splitLength := len(split)
	if splitLength < 2 || (split[0] == "" && split[1] == "") {
		return "", apierr.New("Empty Paths Not Supported", ErrorEmptyPath, nil, nil)
	}

	// get rid of the leading ""
	split = split[1:]
	splitLength = len(split)

	currentSegment := patch.GetRootSegment()
	translatedPath := ""
	finalSeg := ""

	for i, pathSeg := range split {
		val, nextSeg, err := processSegment(currentSegment, pathSeg)
		if err != nil {
			return "", err
		}
		translatedPath = appendToPath(translatedPath, val)

		if val == "-" && nextSeg != nil {
			return "", apierr.New("Invalid Path", ErrorInvalidPath, nil, "'-' must be final path segment")
		}

		if nextSeg != nil && i == splitLength-1 && nextSeg.Optional == false {
			return "", apierr.New("Invalid Path", ErrorInvalidPath, nil, "required path segment missing")
		}

		if nextSeg == nil && i < splitLength-1 {
			return "", apierr.New("Invalid path", ErrorInvalidPath, nil, "path reaches undefined segment: "+p.Path)
		}

		currentSegment = nextSeg
		finalSeg = val
	}

	//Check that the last segment is not "add" and an array index
	if _, e := strconv.Atoi(finalSeg); e == nil && p.Op == Add {
		return "", apierr.New("Unsupported Path", ErrorInvalidPath, nil, "only replace and delete supported for array index. Use '-' to append value to array")
	}

	// Check that it is not removing "-"
	if finalSeg == "-" && p.Op == Remove {
		return "", apierr.New("Invalid Operation", ErrorInvalidOperation, nil, "cannot remove array index of '-'")
	}

	return translatedPath, nil
}

func processSegment(seg *PathSegment, path string) (string, *PathSegment, apierr.Error) {
	var nextSeg *PathSegment
	var val string
	if seg.Wildcard {
		val = path
		path = "*"
	} else {
		st, ok := seg.Values[path]
		if !ok {
			return "", nil, apierr.New("Invalid path", ErrorInvalidSegment, nil, "unknown segement: "+path)
		}
		val = st
	}

	if seg.Children != nil && seg.Children[path] != nil {
		nextSeg = seg.Children[path]
	}

	return val, nextSeg, nil
}

func appendToPath(current string, val string) string {
	if current == "" {
		return val
	}
	return current + "." + val
}

func requiredFurtherSegements(c map[string]*PathSegment) bool {
	for _, seg := range c {
		if !seg.Optional {
			return true
		}
	}
	return false
}

// ValidatePatch makes sure the data provided in the patches is clean
// This does not necessarily mean it is valid based on what model it is attempting to patch
// This routine does NOT validate the value: that is the responsibility of model that implements
// the Patchable interface
func ValidatePatches(patches []Patch) apierr.Error {
	v := validator.New()

	errs := make(map[string]apierr.Error)

	for i, p := range patches {
		v.Validate(validator.NewRequest("op", p.Op, &validator.StringValue{Valid: []string{Add, Remove, Replace}, ErrorCode: ErrorInvalidOperation}))
		v.Validate(validator.NewRequest("path", p.Path, &validator.NotEmpty{Max: util.ConvertInt(50), Regex: util.ConvertString("^\\/.*$")}))

		if !v.IsValid() {
			errs["patch "+strconv.Itoa(i)] = v.Errors()
		}
	}

	if len(errs) != 0 {
		return apierr.New("Invalid patches", validator.ErrorValidation, nil, errs)
	}

	return nil
}

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
