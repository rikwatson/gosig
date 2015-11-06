# gosig
Automatically exported from code.google.com/p/gosig

** gosig is unmaintained at present **

Simple commandline tool for displaying the type signatures of all declarations in a .go file or .go files.

By default, it displays all declarations, private and public.

# Use

```
gosig [flags] files-or-dirs
```

To display only exported declarations invoke with `-D`.

To display only unexported declarations invoke with `-d`.

To display only certain kinds of declarations, use any combination of the following. If none are specified all declarations are shown.

For imports, `-i`.

For constants and variables, `-g`.

For type declarations, -`t`.

For functions, `-f`.

You may specify a regular expression to further filter with the `-m=pattern` switch.
The regular expression is matched only against the name of the declation.

When a directory is specified, all files ending in `.go` are processed, excluding those ending in `_test.go`. To include the `_test.go` files,
specify `-tests`.

For each file parsed, gosig prefaces each file with its path, a colon, and the name of it's package such as
`file.go:main`. To disable this behavior specify the `-p` flag.

To disable printing error messages specify `-q`

# Building

Compile, link, install.
