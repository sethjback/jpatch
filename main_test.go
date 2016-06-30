package jpatch

import (
	"testing"

	"github.com/sethjback/jpatch/jpatcherror"
	"github.com/stretchr/testify/assert"
)

func TestTraceObjectPathString(t *testing.T) {
	assert := assert.New(t)

	bar := &PathSegment{
		Optional: false,
		Wildcard: false,
		Values:   map[string]string{"bar": "bar"}}

	foo := &PathSegment{
		Optional: false,
		Wildcard: false,
		Values:   map[string]string{"foo": "foo", "wild": "wild"},
		Children: map[string]*PathSegment{
			"foo": bar,
			"wild": &PathSegment{
				Optional: true,
				Wildcard: true,
				Children: map[string]*PathSegment{
					"*": bar}}}}

	root := &PathSegment{
		Optional: false,
		Wildcard: false,
		Values:   map[string]string{"rFoo": "foo", "rBar": "bar"},
		Children: map[string]*PathSegment{
			"rFoo": foo,
			"rBar": bar}}

	path, err := traceObjectPathString("/invalid", root)
	assert.Empty(path)
	if assert.NotNil(err) {
		jerr := err.(jpatcherror.Error)
		assert.Equal(jpatcherror.ErrorInvalidSegment, jerr.Code())
	}

	path, err = traceObjectPathString("/rFoo/foo", root)
	assert.Empty(path)
	if assert.NotNil(err) {
		jerr := err.(jpatcherror.Error)
		assert.Equal("required path segment missing", jerr.Details())
	}

	path, err = traceObjectPathString("/rFoo/foo/bar/baz", root)
	if assert.NotNil(err) {
		jerr := err.(jpatcherror.Error)
		assert.Equal("path reaches undefined segment: /rFoo/foo/bar/baz", jerr.Details())
	}

	path, err = traceObjectPathString("/rFoo/wild/-/bar", root)
	if assert.NotNil(err) {
		jerr := err.(jpatcherror.Error)
		assert.Equal(`'-' must be final path segment`, jerr.Details())
	}

	path, err = traceObjectPathString("/rFoo/foo/bar", root)
	assert.Nil(err)
	assert.Equal("bar", path)

	path, err = traceObjectPathString("/rFoo/wild/1/bar", root)
	assert.Nil(err)
	assert.Equal("bar", path)
}

func TestProcessSegment(t *testing.T) {
	assert := assert.New(t)

	seg := &PathSegment{
		Optional: false,
		Wildcard: true,
		Children: map[string]*PathSegment{
			"*": &PathSegment{Optional: false, Wildcard: false, Values: map[string]string{"asdf": "asdf"}}}}

	val, next, err := processSegment(seg, "1")
	assert.Nil(err)
	assert.Equal("1", val)
	assert.Equal(&PathSegment{Optional: false, Wildcard: false, Values: map[string]string{"asdf": "asdf"}}, next)

	seg.Wildcard = false
	seg.Values = map[string]string{"asdf": "jkl;"}

	val, next, err = processSegment(seg, "asdf")
	assert.Nil(err)
	assert.Nil(next)
	assert.Equal("jkl;", val)

	val, next, err = processSegment(seg, "path")
	assert.Nil(next)
	assert.Empty(val)
	if assert.NotNil(err) {
		jerr := err.(jpatcherror.Error)
		assert.Equal(jpatcherror.ErrorInvalidSegment, jerr.Code())
	}
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
