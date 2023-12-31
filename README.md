# proxygen

Proxygen allows you to generate proxies for Go interfaces. These are useful if you want to add middleware/handlers before a call is made to add observability for example.
Even in big codebases this tool should be fairly fast (0-1sec per file) as it only parses the files top-down and doesn't look at nested dependencies unless it has to. The parsing is done using the `golang.org/x/tools/go/packages` package.
You will also notice that this tool does not use the `reflect` package so it fast at runtime as well.

## Installation
Install the `proxygen` tool
```console
go install github.com/panagiotisptr/proxygen/cmd/proxygen@latest
```

## Quickstart

Say that you have an interface that looks like this:
```go
package example

import "context"

type ExampleContext interface {
	context.Context
	ValuesChannel() <-chan interface{}
}
```

You can run this command (in the root of your project) to generate a proxy for that interface:
```console
proxygen \
    --interface github.com/panagiotisptr/demo/example.ExampleContext \
    --package example \
    --name ExampleProxy \
    --output demo/example/example_proxy.go
```

Which will generate this code in `demo/example/example_proxy.go`:

<details>
   <summary>View generated code</summary>
   <br />

```go
// Code generated by proxygen. DO NOT EDIT.
package example

import (
	proxygenInterceptors "github.com/panagiotisptr/proxygen/interceptor"

	importcontextContext5 "time"
)

type ExampleContextProxy struct {
	Implementation ExampleContext
	Interceptors   proxygenInterceptors.InterceptorChain
}

var _ ExampleContext = (*ExampleContextProxy)(nil)

func (this *ExampleContextProxy) ValuesChannel() <-chan interface{} {
	rets := this.Interceptors.Apply(
		[]interface{}{},
		"ValuesChannel",
		func(args []interface{}) []interface{} {
			res0 := this.Implementation.ValuesChannel()

			return []interface{}{
				res0,
			}
		},
	)

	return rets[0].(<-chan interface{})
}

func (this *ExampleContextProxy) Deadline() (
	importcontextContext5.Time,
	bool,
) {
	rets := this.Interceptors.Apply(
		[]interface{}{},
		"Deadline",
		func(args []interface{}) []interface{} {
			res0,
				res1 := this.Implementation.Deadline()

			return []interface{}{
				res0,
				res1,
			}
		},
	)

	return rets[0].(importcontextContext5.Time),
		rets[1].(bool)
}

func (this *ExampleContextProxy) Done() <-chan struct{} {
	rets := this.Interceptors.Apply(
		[]interface{}{},
		"Done",
		func(args []interface{}) []interface{} {
			res0 := this.Implementation.Done()

			return []interface{}{
				res0,
			}
		},
	)

	return rets[0].(<-chan struct{})
}

func (this *ExampleContextProxy) Err() error {
	rets := this.Interceptors.Apply(
		[]interface{}{},
		"Err",
		func(args []interface{}) []interface{} {
			res0 := this.Implementation.Err()

			return []interface{}{
				res0,
			}
		},
	)

	return rets[0].(error)
}

func (this *ExampleContextProxy) Value(
	arg0 any,
) any {
	rets := this.Interceptors.Apply(
		[]interface{}{
			arg0,
		},
		"Value",
		func(args []interface{}) []interface{} {
			res0 := this.Implementation.Value(
				args[0].(any),
			)

			return []interface{}{
				res0,
			}
		},
	)

	return rets[0].(any)
}
```

</details>

If you then have an implementation of that interface you can create the proxy like so:

```go
func NewExampleContext() ExampleContext {
    return &ExampleContext{
        Implementation: ExampleContextImplementation{},
        Interceptors: proxygen.InterceptorChain{
            func(method string, next Handler) Handler {
                return func(args []interface{}) []interface{} {
                    fmt.Println("called:", method)
                    return next(args)
                }
            },
        },
    }
}
```

Even if you don't have an interface for your struct you can easily generate it with [ifacemaker](https://github.com/vburenin/ifacemaker). 
