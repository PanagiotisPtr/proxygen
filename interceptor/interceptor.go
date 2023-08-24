package interceptor

type Interceptor func(
	args []interface{},
	method string,
	impl func([]interface{}) []interface{},
) []interface{}
