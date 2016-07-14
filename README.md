JSON Patch (RFC 6902) Implementation
--------

Provides a structure for allowing models to handle patching logic. The package does basic error checking, but it is up to the implementing model to decide how to handle the actual patches.

### Interface

There are two parts to the interface that an object must implement to use jpatch.

* GetJPatchRootSegment

  This provides a description of the paths (RFC 6901) within the object that are patchable. By defining this outright the package can avoid reflection and validate the paths attempting to be patched are valid.

* ValidateJPatchPatches

  This allows the model to look at the patches, validate the new values or operations, and modify the patches as needed.

  For example, one of the main motivations for creating this package was to handle REStful API patch operations on object stored in AWS Dynamodb. In the case of lists, there is no easy way to support RFC 6902 add, which indicates an item should be inserted into a list at the index specified in the path. Using the ValidateJPatchPatches interface, a model can modify the existing (or new) list as needed, then return a new patch that contains the entire list to be written back to Dynamodb.

### PathSegments

Path Segments define each step along a path within the model. There are 4 parts:

1. Optional

  Boolean - this indicates whether this segment is optional, and is used to reject patches that don't reach it.

  Example: if an object contains the path /foo/bar/baz, and the the /baz segment is NOT optional, a patch that attempts to modify /foo/bar will be rejected.

2. Wildcard

  Boolean - indicates whether the segment can contain any value, and is useful for maps and lists.

  Example: when modifying an array, the index could be any number representing an item in the array (/foo/1/bar, /foo/2/bar, /foo/3, etc). Wildcard indicates that there is no pre-defined value for this segement in the Values map

3. Values

  This map does three things. First, it is used to specify all the possible valid values for this segment (e.g. if an object has properties foo, bar, and baz, the values map will have 3 items: foo, bar, and baz). Wildcard values are specified using *

  Second, it provides a way to substitute value names if they are different in the DB from the JSON. Example: if your API exposes a JSON object with a property of fooBar, but stores that internally in the DB as foo_bar, the `Name` in `PathValue` should be foo_bar. When someone sends a patch for /fooBar, after processing the path will be /foo_bar

  Finally, this provides a way for specifying which operations are valid on this value. If you have certain values that cannot be removed, specifying `Replace` in the `SupportedOps` slice will reject all patches that attempt to add or remove values. This is a way reject early patches that are invalid, before they get to the model's validation routine.

4. Children

  This is the map of all possible children under the current path. The key is the same as the Values key.

  For wildcard values, use *

  Example: foo contains another object baz, with it's own properties. The children entry under "baz" would contain a PathSegment that defines how it can be patched. If this child segment is not optional, then any patch that stops short of this segment will fail.

  This is useful for encapsulating patch logic. If you have an object that contains other objects, you can simply call GetJPatchRootSegment on the child object and provide that segment here.

### Implementation

Jpatch relies on the objects to validate the operations and values, and thus it can't implement the full RFC spec (e.g. failing when an array index for and Add operation is longer than the array). The goal was twofold: allow objects the flexibility to handle error situations (e.g. opting to append an item onto an array when the index is greater than the total number of items), and avoid having to know about the objects themselves (i.e avoid reflection for performance reasons).

The result is a framework that allows you to make objects patchable by implementing two functions, and remains fairly flexible in how you actually handle the patching.
