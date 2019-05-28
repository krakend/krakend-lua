package proxy

import (
	"fmt"

	"github.com/alexeyco/binder"
)

func Example_RegisterBackendModule() {
	bindr := binder.New(binder.Options{
		SkipOpenLibs:        true,
		IncludeGoStackTrace: true,
	})

	registerHTTPRequest(bindr)

	if err := bindr.DoString(sampleLuaCode); err != nil {
		fmt.Println(err)
	}

	// output:
	// lua http test
	// 200
	// application/json
	// {
	//     "uno":"el brikindans",
	//     "dos":"el crusaíto",
	//     "tres":"el maiquelyason",
	//     "cuatro":"el robocop"
	// }
}

const sampleLuaCode = `
print("lua http test")
local r = http_response.new('http://www.mocky.io/v2/5cec657f330000165f6d7a83')
print(r:statusCode())
print(r:headers('Content-Type'))
print(r:body())
`
