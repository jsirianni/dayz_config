// Package batchvalidate provides conservative, static validation of Windows
// batch-file source. It never executes the source and deliberately leaves
// undocumented or runtime-dependent behavior unjudged.
//
// A valid Result means that no supported, documented rule was violated. It is
// not a guarantee that cmd.exe will execute the source successfully.
package batchvalidate
