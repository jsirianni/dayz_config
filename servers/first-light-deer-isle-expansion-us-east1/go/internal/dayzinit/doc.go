// Package dayzinit validates DayZ server mission init.c source without
// compiling or executing it.
//
// Validation uses a pure-Go compiler front end: source and preprocessor
// checks, lexical analysis, parsing, conservative static semantics, and the
// DayZ mission entry-point contract. Logic validation is limited to defects
// provable from one file, such as invalid control flow, duplicate local
// declarations, incompatible known primitive expressions, local call-shape
// errors, and local inheritance cycles. Engine and mod symbols that are not
// declared in the file are intentionally treated as opaque external symbols.
//
// ValidateFile requires a regular file whose basename is init.c. Basename
// comparison is ASCII case-insensitive so paths behave consistently on
// Windows and Unix hosts. UTF-8 BOMs and LF or CRLF line endings are accepted.
// Scripts are never executed, includes are never loaded, and validation makes
// no network or DayZ-runtime calls.
//
// Diagnostic codes are stable identifiers. Message and hint wording may be
// clarified in compatible releases. Deliberately permissive areas include
// unresolved external APIs and uncommon syntax whose validity cannot be
// established from official DayZ evidence.
//
// Syntax decisions were informed by the Bohemia Interactive DayZ Enforce
// Script Syntax and Modding Basics documentation, official DayZ Central
// Economy mission examples, and the MIT-licensed tree-sitter-enforce grammar
// as a secondary reference. No upstream parser implementation or Bohemia
// source corpus is included in this package.
package dayzinit
