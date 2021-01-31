# 1.0.2 (June 17, 2020)

* The YAML decoder now follows the YAML specification more closely when parsing
  numeric values.
  ([#6](https://github.com/zclconf/go-cty-yaml/pull/6))

# 1.0.1 (July 30, 2019)

* The YAML decoder is now correctly treating quoted scalars as verbatim literal
  strings rather than using the fuzzy type selection rules for them. Fuzzy
  type selection rules still apply to unquoted scalars.
  ([#4](https://github.com/zclconf/go-cty-yaml/pull/4))

# 1.0.0 (May 26, 2019)

Initial release.
