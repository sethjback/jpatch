package jpatch

import (
	"testing"

	"github.com/sethjback/jpatch/jpatcherror"
	"github.com/stretchr/testify/assert"
)

func TestShift(t *testing.T) {
	assert := assert.New(t)

	p := Patch{
		Op:    Add,
		Value: 1234,
		Path:  "/l1/l2/l3",
		From:  ""}

	assert.Equal(Patch{
		Op:    Add,
		Value: 1234,
		Path:  "/l2/l3",
		From:  ""}, p.Shift())

	p.From = "/l1/l3"
	assert.Equal(Patch{
		Op:    Add,
		Value: 1234,
		Path:  "/l2/l3",
		From:  "/l3"}, p.Shift())

	p.From = "/l3"
	assert.Equal(Patch{
		Op:    Add,
		Value: 1234,
		Path:  "/l2/l3",
		From:  "/"}, p.Shift())
}

func TestTraceObjectPathString(t *testing.T) {
	assert := assert.New(t)

	bar := &PathSegment{
		Optional: false,
		Wildcard: false,
		Values:   map[string]*PathValue{"bar": &PathValue{Name: "bar"}}}

	barOp := &PathSegment{
		Optional: true,
		Wildcard: false,
		Values:   map[string]*PathValue{"bar": &PathValue{Name: "bar"}}}

	foo := &PathSegment{
		Optional: false,
		Wildcard: false,
		Values:   map[string]*PathValue{"foo": &PathValue{Name: "foo"}, "wild": &PathValue{Name: "wild"}, "wild2": &PathValue{Name: "wild2"}},
		Children: map[string]*PathSegment{
			"foo": bar,
			"wild2": &PathSegment{
				Optional: true,
				Wildcard: true,
				Children: map[string]*PathSegment{
					"*": barOp}},
			"wild": &PathSegment{
				Optional: true,
				Wildcard: true,
				Children: map[string]*PathSegment{
					"*": bar}}}}

	root := &PathSegment{
		Optional: false,
		Wildcard: false,
		Values:   map[string]*PathValue{"rFoo": &PathValue{Name: "foo"}, "rBar": &PathValue{Name: "bar"}},
		Children: map[string]*PathSegment{
			"rFoo": foo,
			"rBar": bar}}

	path, lastVal, err := traceObjectPathString("/invalid", Add, root)
	assert.Empty(path)
	if assert.NotNil(err) {
		jerr := err.(jpatcherror.Error)
		assert.Equal(jpatcherror.ErrorInvalidSegment, jerr.Code())
	}

	path, lastVal, err = traceObjectPathString("/rFoo/foo", Add, root)
	assert.Empty(path)
	if assert.NotNil(err) {
		jerr := err.(jpatcherror.Error)
		assert.Equal("required path segment missing", jerr.Details())
	}

	path, lastVal, err = traceObjectPathString("/rFoo/foo/bar/baz", Add, root)
	if assert.NotNil(err) {
		jerr := err.(jpatcherror.Error)
		assert.Equal("path reaches undefined segment: /rFoo/foo/bar/baz", jerr.Details())
	}

	path, lastVal, err = traceObjectPathString("/rFoo/wild/-/bar", Add, root)
	if assert.NotNil(err) {
		jerr := err.(jpatcherror.Error)
		assert.Equal(`'-' must be final path segment`, jerr.Details())
	}

	path, lastVal, err = traceObjectPathString("/rFoo/foo/bar", Add, root)
	assert.Nil(err)
	assert.Equal("/foo/foo/bar", path)

	path, lastVal, err = traceObjectPathString("/rFoo/wild/1/bar", Add, root)
	assert.Nil(err)
	assert.Equal("/foo/wild/1/bar", path)

	path, lastVal, err = traceObjectPathString("/rFoo/wild2/-", Add, root)
	assert.Nil(err)
	assert.Equal("/foo/wild2/-", path)
	assert.Equal("wild2", lastVal.Name)

	path, lastVal, err = traceObjectPathString("/rFoo/wild2/1/baz", Add, root)
	assert.NotNil(err)

	path, lastVal, err = traceObjectPathString("/rFoo/wild2/1/bar", Add, root)
	assert.Nil(err)
}

func TestProcessSegment(t *testing.T) {
	assert := assert.New(t)

	seg := &PathSegment{
		Optional: false,
		Wildcard: true,
		Children: map[string]*PathSegment{
			"*": &PathSegment{Optional: false, Wildcard: false, Values: map[string]*PathValue{"asdf": &PathValue{Name: "asdf"}}}}}

	val, next, err := processSegment(seg, "1")
	assert.Nil(err)
	assert.Equal("1", val.Name)
	assert.Equal(&PathSegment{Optional: false, Wildcard: false, Values: map[string]*PathValue{"asdf": &PathValue{Name: "asdf"}}}, next)

	seg.Wildcard = false
	seg.Values = map[string]*PathValue{"asdf": &PathValue{Name: "jkl;"}}

	val, next, err = processSegment(seg, "asdf")
	assert.Nil(err)
	assert.Nil(next)
	assert.Equal("jkl;", val.Name)

	val, next, err = processSegment(seg, "path")
	assert.Nil(next)
	assert.Empty(val)
	if assert.NotNil(err) {
		jerr := err.(jpatcherror.Error)
		assert.Equal(jpatcherror.ErrorInvalidSegment, jerr.Code())
	}
}

func TestValidatePath(t *testing.T) {
	assert := assert.New(t)
	bar := &PathSegment{
		Optional: false,
		Wildcard: false,
		Values:   map[string]*PathValue{"bar": &PathValue{Name: "bar", SupportedOps: []string{Add, Remove}}, "baz": &PathValue{Name: "baz", SupportedOps: []string{Copy, Remove}}}}

	path, err := validatePath("/bar", Test, bar)
	assert.NotNil(err)
	assert.Empty(path)

	path, err = validatePath("/baz", Copy, bar)
	assert.Nil(err)
	assert.Equal("/baz", path)
}

func TestValidatePatch(t *testing.T) {
	assert := assert.New(t)

	p := Patch{Op: "invalid"}

	err := validatePatch(p)
	if assert.NotNil(err) {
		jerr := err.(jpatcherror.Error)
		assert.Equal(jpatcherror.ErrorInvalidOperation, jerr.Code())
	}

	p.Op = Add
	err = validatePatch(p)
	if assert.NotNil(err) {
		jerr := err.(jpatcherror.Error)
		assert.Equal(jpatcherror.ErrorInvalidPath, jerr.Code())
		assert.Equal("Empty Paths Not Supported", jerr.Message())
	}

	p.Path = "/foo/bar"
	p.Op = Move
	err = validatePatch(p)
	if assert.NotNil(err) {
		jerr := err.(jpatcherror.Error)
		assert.Equal(jpatcherror.ErrorInvalidPath, jerr.Code())
		assert.Equal("From path required", jerr.Message())
	}

	p.Op = Test
	err = validatePatch(p)
	if assert.NotNil(err) {
		jerr := err.(jpatcherror.Error)
		assert.Equal(jpatcherror.ErrorInvalidValue, jerr.Code())
		assert.Equal("Value required", jerr.Message())
	}
}
