// Package rpc contains the API that can be used to serve Sentinel
// plugins over an RPC interface. Sentinel supports consuming plugins
// across RPC with the requirement that the RPC must happen over a completely
// reliable network (effectively a local network).
//
// ## Object Plugins
//
// Object plugins allow Sentinel values to be served over a plugin interface.
// This implements the object.External interface exported by lang/object.
//
// There are limitations to the types of values that can be returned when
// this is served over a plugin:
//
//     * All Go primitives and collections that the External interface
//       allows may be returned, including structs.
//
//     * All primitive and collection Object implementations may be returned.
//
//     * ExternalObj, External may not yet be returned. We plan to allow this.
//
package rpc
