# Batch validator rule ledger

This ledger records the authority for every diagnostic that can make a result
invalid. It was reviewed against the linked Microsoft documentation on
2026-08-14. `Valid()` means only that no rule in this supported subset was
violated; it is not a promise that `cmd.exe` will execute a file successfully.

Evidence levels follow the package design:

- **A** — current Microsoft Learn documentation.
- **B** — historical Microsoft documentation linked from current Learn pages.

Microsoft Q&A, third-party grammars, other command interpreters, and observed
`cmd.exe` behavior are not normative sources for hard errors.

## Sources

- [CMD](https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/cmd) (A)
- [Command-Line Syntax Key](https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/command-line-syntax-key) (A)
- [IF](https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/if) (A)
- [FOR](https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/for) (A)
- [SET](https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/set_1) (A)
- [CALL](https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/call) (A)
- [GOTO](https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/goto) (A)
- [SETLOCAL](https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/setlocal) (A)
- [ENDLOCAL](https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/endlocal) (A)
- [SHIFT](https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/shift) (A)
- [ECHO](https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/echo) (A)
- [REM](https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/rem) (A)
- [Using command redirection operators](https://learn.microsoft.com/en-us/previous-versions/windows/it-pro/windows-xp/bb490982(v=technet.10)) (B)

## Hard rules

| Rule ID | Construct | Documented rule used by this package | Evidence | Diagnostic | Deterministic tests |
|---|---|---|---|---|---|
| BV-SHELL-001 | command chains | `&`, `&&`, `||`, and `|` join commands and therefore require command operands | CMD | `BAT1001` | `TestEveryHardDiagnostic`, `TestRecoveryFindsLaterIndependentErrors` |
| BV-SHELL-002 | command groups | parentheses form paired command groups; a group opener requires a close | CMD, Syntax Key | `BAT1002` | `TestEveryHardDiagnostic` |
| BV-SHELL-003 | command groups | a closing group delimiter requires a corresponding opener | CMD, Syntax Key | `BAT1003` | `TestEveryHardDiagnostic` |
| BV-IF-001 | IF | each documented form requires its condition operands and a command | IF syntax and parameter table | `BAT2001` | `TestEveryHardDiagnostic`, `TestGeneratedCombinations` |
| BV-IF-002 | IF comparison | extension comparison operators are `EQU`, `NEQ`, `LSS`, `LEQ`, `GTR`, and `GEQ` | IF | `BAT2002` | `TestEveryHardDiagnostic`, `TestDocumentedAcceptedForms` |
| BV-IF-003 | ELSE | ELSE must be on the same physical line as the command after IF | IF remarks | `BAT2003` | `TestEveryHardDiagnostic`, `if_bad_else_placement.bat` |
| BV-IF-004 | numeric conditions | literal `ERRORLEVEL` and `CMDEXTVERSION` operands use the documented number slot | IF | `BAT2004` | `TestEveryHardDiagnostic` |
| BV-FOR-001 | FOR variable | batch-file FOR syntax uses `%%`, while interactive syntax uses `%` | FOR parameter table | `BAT3001` | `TestEveryHardDiagnostic`, `for_single_percent.bat` |
| BV-FOR-002 | FOR keywords | `IN` is mandatory | FOR remarks | `BAT3002` | `TestEveryHardDiagnostic`, `for_missing_in.bat` |
| BV-FOR-003 | FOR set | the set parentheses are required | FOR parameter table | `BAT3003` | `TestEveryHardDiagnostic` |
| BV-FOR-004 | FOR keywords | `DO` is mandatory | FOR remarks | `BAT3004` | `TestEveryHardDiagnostic`, `for_missing_do.bat` |
| BV-FOR-005 | FOR /L | the documented tuple contains start, step, and end values | FOR /L syntax | `BAT3005` | `TestEveryHardDiagnostic`, `TestGeneratedCombinations` |
| BV-FOR-006 | FOR modes and /F | modes are `/D`, `/R`, `/L`, `/F`; parsing keywords are `eol=`, `skip=`, `delims=`, `tokens=`, and `usebackq`; `eol=` names one character | FOR | `BAT3006` | `TestEveryHardDiagnostic`, `TestFORParsingHelpers` |
| BV-FOR-007 | FOR body | the command after DO is required | FOR parameter table | `BAT3007` | `TestEveryHardDiagnostic` |
| BV-FOR-008 | FOR /F allocation | `tokens=` allocates subsequent variables and a valid FOR variable terminates `%~` syntax; a statically proven allocation cannot pass the applicable alphabetic boundary | FOR variable substitution and `tokens=` tables | `BAT3008` | `TestEveryHardDiagnostic` |
| BV-FOR-009 | nested FOR | no more than 52 replaceable variables can be active at one time; this is checked only for syntactically proven nesting | FOR remarks | `BAT3009` | `TestNestedFORActiveVariableLimit` |
| BV-SET-001 | SET /P | `/P` uses `<variable>=<promptString>` | SET syntax | `BAT4001` | `TestEveryHardDiagnostic` |
| BV-SET-002 | SET /A | `/A` requires a numerical expression | SET syntax | `BAT4002` | `TestEveryHardDiagnostic`, `TestArithmeticDynamicExpansionAndResourceLimit` |
| BV-SET-003 | SET /A | grouping and documented unary, binary, assignment, and separator operators require operands in their documented roles | SET operator table | `BAT4003` | `TestEveryHardDiagnostic`, `TestSetArithmeticNumbersAndErrors` |
| BV-SET-004 | SET /A numbers | numbers are decimal, `0x` hexadecimal, or leading-zero octal | SET remarks | `BAT4004` | `TestEveryHardDiagnostic`, `TestSetArithmeticNumbersAndErrors` |
| BV-CALL-001 | CALL | a file or label target is required | CALL syntax | `BAT5001` | `TestEveryHardDiagnostic` |
| BV-CALL-002 | CALL file target | a literal target with an explicit extension must use `.bat` or `.cmd` | CALL parameter table | `BAT5002` | `TestEveryHardDiagnostic`, `call_bad_static_target.bat` |
| BV-PARAM-001 | batch parameter modifier | `%~` modifiers cannot be used with `%*` | CALL batch-parameter table | `BAT5003` | `TestEveryHardDiagnostic`, `TestExpansionRecognition` |
| BV-GOTO-001 | GOTO | a target label parameter is required | GOTO syntax | `BAT6001` | `TestEveryHardDiagnostic` |
| BV-GOTO-002 | GOTO | a statically specified target must match a label; `:EOF` is special when extensions may be enabled | GOTO remarks | `BAT6002` | `TestEveryHardDiagnostic`, `TestStaticTargetsAndEOFFeatureState` |
| BV-GOTO-003 | GOTO label parameter | spaces are allowed, but semicolon and equals separators are not | GOTO remarks | `BAT6003` | `TestEveryHardDiagnostic` |
| BV-LOCAL-001 | SETLOCAL | literal options are `enableextensions`, `disableextensions`, `enabledelayedexpansion`, and `disabledelayedexpansion` | SETLOCAL syntax | `BAT7001` | `TestEveryHardDiagnostic`, `setlocal_bad_literal_option.bat` |
| BV-SHIFT-001 | SHIFT | the extension `/N` value is in the literal range 0 through 8 | SHIFT | `BAT7002` | `TestEveryHardDiagnostic`, `shift_out_of_range.bat` |
| BV-FEATURE-001 | extension forms | the applicable IF, FOR, CALL-label, and SHIFT forms require command extensions | CMD plus command-specific pages | `BAT7003` | `TestEveryHardDiagnostic`, `TestFeatureStateIsConservativeAndScoped` |
| BV-REDIR-001 | redirection | `<`, `>`, and `>>` require a source/destination target | redirection documentation | `BAT8001` | `TestEveryHardDiagnostic`, `TestChainOperatorsAndRedirectionMetadata` |
| BV-REDIR-002 | handle redirection | `<&` and `>&` name one of the documented handles 0 through 9 | redirection documentation | `BAT8002` | `TestEveryHardDiagnostic`, `TestChainOperatorsAndRedirectionMetadata` |

`BAT9001`, `BAT9002`, and `BAT9003` are informational analysis-limit
diagnostics. They never make a result invalid and therefore are not hard rules.

## Conservative conflict decisions

### CALL and redirection

The CALL page says not to use pipes or redirection with CALL. The ENDLOCAL page
contains a Microsoft example using `call superapp>c:\superapp.out`. This package
accepts CALL with shell redirection and records no hard error for the disputed
combination.

### Extensionless CALL targets

The CALL parameter table says the filename must have a `.bat` or `.cmd`
extension, while examples on the same page use `call checknew`. An explicit,
non-batch extension such as `helper.exe` is rejected. An extensionless static
target is accepted but makes `FullyValidated` false.

The CALL page does not explicitly document the failure mode for an absent static
CALL label. Unlike GOTO, this package does not invent a hard missing-label rule;
the unresolved label is reported only as `BAT9002` when unsupported reporting is
enabled.

### FOR variable wording

The FOR parameter table says variables are alphabetic, while the remarks say any
character other than 0 through 9 can be used and also describe multiple values.
The parser accepts the broader clearly documented form. Alphabetic boundary
checks are applied only when a starting ASCII letter makes the `/F tokens=`
allocation provable.

### SET /A operator presentation

The current SET page's rendered operator table and the following prose do not
present logical and bitwise operators consistently. The parser accepts the union
of operators explicitly shown or named, but does not emit precedence diagnostics.
Unary plus is parsed tolerantly as unsupported because it is not in the displayed
unary operator row.

### Quotes, escaping, and expansion

An unmatched quote is not treated like an unterminated Go/C string. It is
preserved and makes the analysis incomplete. Dynamic `%NAME%`, `!NAME!`, target,
tuple, option, and handle values are never resolved or reparsed. Unknown modifier
sequences, multi-digit redirection handles, unknown commands, and undocumented
argument forms remain opaque rather than invalid.

### Feature state and control flow

Initial extension and delayed-expansion state is tri-state. Explicit SETLOCAL
options can prove a local state and ENDLOCAL restores it. After control flow that
the package does not model exactly, the state is merged to unknown. `BAT7003` is
emitted only while the state is provably disabled; unknown never means disabled.

## Explicitly unvalidated behavior

- external executable argument grammars;
- file, directory, wildcard, registry, and environment state;
- post-expansion command reparsing and variable values;
- exact unmatched-quote behavior;
- undocumented multi-digit handles;
- extensionless CALL dispatch and absent CALL-label behavior;
- command behavior outside the built-ins implemented by this package;
- execution results, exit codes, `ERRORLEVEL`, and arithmetic values.
