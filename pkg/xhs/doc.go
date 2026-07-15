// Package xhs provides Xiaohongshu (小红书) search and note-reading as
// first-class tools, independent of the generic browser primitives.
//
// All XHS-specific logic lives here. The generic browser_extract no longer
// carries any XHS special-casing. xhs.Client drives a stealth browser page
// (supplied by the caller via the Browser interface) to read server-rendered
// note state and to call XHS's own search API signed from within the page.
package xhs
