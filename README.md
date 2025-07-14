# esi [![Go Reference](https://pkg.go.dev/badge/github.com/nussjustin/esi.svg)](https://pkg.go.dev/github.com/nussjustin/esi) [![Lint](https://github.com/nussjustin/esi/actions/workflows/golangci-lint.yml/badge.svg)](https://github.com/nussjustin/esi/actions/workflows/golangci-lint.yml) [![Test](https://github.com/nussjustin/esi/actions/workflows/test.yml/badge.svg)](https://github.com/nussjustin/esi/actions/workflows/test.yml)

Package esi provides functions for parsing and processing [ESI](https://www.w3.org/TR/esi-lang/) from arbitrary inputs.

## Examples

### Parsing documents containing ESI

To parse ESI from arbitrary `io.Reader`, first create a new [esi.Parser][0] using [esi.NewParser][8].

```go
parser := esi.NewParser(reader)
```

It is also possible to initialize or re-use a parser from an existing instance, using [Parser.Reset][9]:

```
var parser esi.Parser
parser.Reset(reader)
```

Once the parser has been created, either use [Parser.Next][10] to read each node from the input, or, use
[Parser.All][11] to directly iterate over the stream of nodes:

```go
for node, err := parser.All {
    if err != nil {
        panic(err)
    }
}
```

### Processing ESI instructions

Once the input is parsed, it can be used processed using the [esiproc][1] package.

To do this first create a [esiproc.Processor][2] using [esiproc.New][3] and configure it to fetch data for ESI includes.

The [esihttp][13] package implements a custom type that can be used to resolve includes via HTTP.

```go
proc := esiproc.New(
    esiproc.WithClient(&esihttp.Client{}),
    esiproc.WithClientConcurrency(4))
```

Once created the processor can be used to process multiple sets of nodes, both sequentially and concurrently.

To actually process some data, call the [Processor.Process][12] method. The method takes a `context.Context`, an
`io.Writer` that will be written to and a slice of ESI nodes. 

Assuming the variable `parser` contains a `esi.Parser`, one could process its nodes like this:

_NOTE: The requirement to pass a whole slice of nodes is currently a limitation and is expected to be lifted in the
future._

_When that happens, the signature of `Processor.Process` will likely be changed to take an
`iter.Seq2[esi.Node, error]` instead._

```go
var nodes []esi.Node

for node, err := parser.All {
    if err != nil {
        panic(err)	
    }
	
    nodes = append(nodes, node)
}

var buf bytes.Buffer

if err := proc.Process(ctx, &buf, nodes); err != nil {
    panic(err)
}

// Output the processed content
fmt.Println(buf.Bytes())
```

By default, `<esi:when/>` elements will result in an error. To fix this, an [esiproc.EvalFunc][4] must be configured and
passed to the `esiproc.Processor` using the [esiproc.WithEvalFunc][6] function.

Similarly, variables in the `alt` or `src` attributes of `<esi:include/>` tags are not interpolated by default. This can
be enabled by providing an [esiproc.InterpolateFunc][14] using [esiproc.WithInterpolateFunc][15].

The [esiexpr][5] package implements an `Env` type that provides methods for evaluating ESI expressions for use with
`<esi:when/>` elements as well as the interpolation of variable in arbitrary strings.

Once an `Env` has been created its method can be used with `esiproc.WithEvalFunc` and `esiproc.WithInterpolateFunc` to
configure ann `esiproc.Proc` like this:

```go
myEnv := &esiproc.Env{
    LookupVar: func(ctx context.Context, name string, key *string) (ast.Value, error) {
        // ...lookup name and return the value
        return val, nil
    },
}

proc := esiproc.New(
    esiproc.WithEvalFunc(myEnv.Eval),
    esiproc.WithInterpolateFunc(myEnv.Interpolate))
```

## Contributing
Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

Please make sure to update tests as appropriate.

## License
[MIT](https://choosealicense.com/licenses/mit/)

[0]: https://pkg.go.dev/github.com/nussjustin/esi/#Parse
[1]: https://pkg.go.dev/github.com/nussjustin/esi/esiproc/
[2]: https://pkg.go.dev/github.com/nussjustin/esi/esiproc/#Processor
[3]: https://pkg.go.dev/github.com/nussjustin/esi/esiproc/#New
[4]: https://pkg.go.dev/github.com/nussjustin/esi/esiproc/#EvalFunc
[5]: https://pkg.go.dev/github.com/nussjustin/esi/esiexpr/
[6]: https://pkg.go.dev/github.com/nussjustin/esi/esiproc/#WithEvalFunc
[8]: https://pkg.go.dev/github.com/nussjustin/esi/#NewParser
[9]: https://pkg.go.dev/github.com/nussjustin/esi/#Parser.Reset
[10]: https://pkg.go.dev/github.com/nussjustin/esi/#Parser.Next
[11]: https://pkg.go.dev/github.com/nussjustin/esi/#Parser.All
[12]: https://pkg.go.dev/github.com/nussjustin/esi/esiproc/#Processor.Process
[13]: https://pkg.go.dev/github.com/nussjustin/esi/esihttp/
[14]: https://pkg.go.dev/github.com/nussjustin/esi/esiproc/#InterpolateFunc
[15]: https://pkg.go.dev/github.com/nussjustin/esi/esiproc/#WithInterpolateFunc