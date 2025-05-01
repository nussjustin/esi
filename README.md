# go-esi [![Go Reference](https://pkg.go.dev/badge/github.com/nussjustin/esi.svg)](https://pkg.go.dev/github.com/nussjustin/esi) [![Lint](https://github.com/nussjustin/esi/actions/workflows/golangci-lint.yml/badge.svg)](https://github.com/nussjustin/esi/actions/workflows/golangci-lint.yml) [![Test](https://github.com/nussjustin/esi/actions/workflows/test.yml/badge.svg)](https://github.com/nussjustin/esi/actions/workflows/test.yml)

Package esi provides functions for parsing and processing [ESI](https://www.w3.org/TR/esi-lang/) from arbitrary inputs.

## Examples

### Parsing documents containing ESI

To parse ESI from arbitrary input, use the [esi.Parse][0] function.

```go
nodes, err := esi.Parse(`<p>Hello <esi:include src="/me"/></p>`)
if err != nil {
    panic(err)
}
```

### Processing ESI instructions

Once the input is parsed, it can be used processed using the [esiproc][1] package.

To do this first create a [esiproc.Processor][2] using [esiproc.New][3] and configure it to fetch data for ESI includes:

```go
inc := esiproc.IncludeFunc(func(ctx context.Context, urlStr string, _ map[string]string) ([]byte, error) {
    req, err := http.NewRequestWithContext(ctx, "GET", inc.Source, nil)
    if err != nil {
        return nil, err
    }
    
    resp, err := http.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        // do something...
    }
    
    return io.ReadAll(resp.Body)
})

proc := esiproc.New(
    // Allow up to 4 concurrent HTTP requests
    esiproc.WithIncludeConcurrency(4),
    esiproc.WithIncludeFunc(inc))
```

Once created the processor can be used to process multiple sets of nodes, both sequentially and concurrently.

Now the processor can be used like this:

```go
var buf bytes.Buffer

if err := proc.Process(ctx, &buf, nodes); err != nil {
    panic(err)
}

// Output the processed content
fmt.Println(buf.Bytes())
```

It is also possible to provide an [esiproc.Env][4] to enable the use of variables in URLs as well as
`<esi:when test="...">` conditions.

The [esiexpr][5] package implements such an `Env` that implements the ESI variable and expression syntax. To use it,
simply create an instance and pass it to `esiproc.New` via [esiproc.WithEnv][6]:

```go
myEnv := &esiproc.Env{
    LookupVar: func(ctx context.Context, name string, key *string) (ast.Value, error) {
        // ...lookup name and return the value
        return val, nil
    },
}

proc := esiproc.New(
    esiproc.WithEnv(myEnv),
    // Allow up to 4 concurrent HTTP requests
    esiproc.WithFetchConcurrency(4),
    esiproc.WithFetchFunc(fetch))
```

When finished with the processor, call [Processor.Release][7] to release all resources.

## Contributing
Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

Please make sure to update tests as appropriate.

## License
[MIT](https://choosealicense.com/licenses/mit/)

[0]: https://pkg.go.dev/github.com/nussjustin/esi/#Parse
[1]: https://pkg.go.dev/github.com/nussjustin/esi/esiproc/
[2]: https://pkg.go.dev/github.com/nussjustin/esi/esiproc/#Processor
[3]: https://pkg.go.dev/github.com/nussjustin/esi/esiproc/#New
[4]: https://pkg.go.dev/github.com/nussjustin/esi/esiproc/#Env
[5]: https://pkg.go.dev/github.com/nussjustin/esi/esiexpr/
[6]: https://pkg.go.dev/github.com/nussjustin/esi/esiproc/#WithEnv
[7]: https://pkg.go.dev/github.com/nussjustin/esi/esiproc/#Processor.Release