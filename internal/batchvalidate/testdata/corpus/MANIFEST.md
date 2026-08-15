# Corpus manifest

| Fixture | Origin | License | Reason | Expected status |
|---|---|---|---|---|
| `repository_service_sample.bat` | Authored for this repository from the shape of its service-management scripts; no copied source | Repository terms | Realistic multi-line service setup with feature state, IF, grouping, and delayed expansion | valid-partial |

No third-party batch source is vendored. This avoids importing code with uncertain
licensing merely to enlarge a parser corpus; deterministic generated cases cover
the syntax combinations separately.
