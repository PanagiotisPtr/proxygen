package interceptor

type Handler func(
	args []interface{},
) []interface{}

type Interceptor func(
	method string,
	next Handler,
) Handler

type InterceptorChain []Interceptor

func (chain InterceptorChain) Apply(
	args []interface{},
	method string,
	h Handler,
) []interface{} {
	for i := len(chain) - 1; i >= 0; i-- {
		h = chain[i](method, h)
	}

	return h(args)
}
