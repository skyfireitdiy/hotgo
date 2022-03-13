# hotgo - Hot Patch support for Go

[中文版](./README_cn.md)

Hotgo is a Go language pack used to provide hot patching capabilities for applications.

## Getting started

1. Enable applications to support hot patches. 

Start the hot patch http server in the application:

```go
httpServer := hotgo.HPHttpServer()
l, _ := net.Listen("tcp", ":8080")
http.Serve(l, httpServer)
```

Or rpc server:

```go
rpcserver := hotgo.HPRpcServer()
rpcserver.HandleHTTP("/hp", "/hpdebug")
http.ListenAndServe(":8080", rpcserver)
```

In order to prevent the compiler from optimizing the symbols, it is recommended to add the parameter `- gcflags= "- N-l"` when compiling.

2. Make hot patch file.

Hot patch files are actually so files supported by go plugin, which can be compiled using the command parameter `--buildmode= plugin`.

3. Use http requests or rpc calls to load hot patches.

Take http as an example:

```bash
curl -d '{
    "hp_file": "./hp.so",    
    "replace_config": {
        "NewFunc1":"main.func1"
    },
    "ref_config": {
        "Global": "main.globalValue",
        "Func2Ref": "main.func2"
    }
}' http://127.0.0.1:8080/hp/v1/load
```

## Interface description

1. Hotgo provides the following function interfaces

- func LoadHP(config Config) (hpID string, errret error)
  
  A hot patch is loaded with the specified configuration, and a hot patch ID is returned for success, and an error message is returned for failure.

- func UnloadHP(hpID string) (errret error)

  Uninstall the hot patch for the specified ID, and the failure returns the cause of the error.

- func HPHttpServer() http.Handler

  Create a hot patch HttpServer that uses the http protocol to access the hot patch load and unload interface (this interface returns only one Handler and needs to be serviced by the self-listening port).
  
- func HPRpcServer() *rpc.Server

  Create a RpcServer to access the hot patch load and unload interface using the http protocol. 
  
  This API is the same as HPHttpServer, and only returns one Server. You need to monitor and provide services by yourself.

2. Hotgo provides the following exposed structures

```go
type Config struct {
	HPFile        string            `json:"hp_file"`
	ReplaceConfig map[string]string `json:"replace_config"`
	RefConfig     map[string]string `json:"ref_config"`
}
```

The configuration required to load the hot patch.

- HPFile: the path where hot patch files are stored. 
- ReplaceConfig: replace the mapping table. 

The mapped key is the symbol name in the hot patch and the value is the symbol name that needs to be replaced in the running program. 

- RefConfig: reference mapping table. 
    
The mapped key is the symbol name in the hot patch, and the value is the symbol name to be referenced in the running program.

```go
type LoadRequest Config
```

LoadRequest is an alias for the Config structure and is used for the parameters of the http or rpc interface.

```go
type UnloadHPRequest struct {
	HPID string `json:"hp_id"`
}
```

UnloadHPRequest is the type of parameter required by the hot patch uninstall interface. 
HPID is the value returned by the load interface.

```go
type HPResponse struct {
	HPID  string `json:"hp_id"`
	Error string `json:"error"`
}
```

The return type of the http or rpc interface. 
HPID is a hot patch ID returned in the hot patch loading interface, which makes no sense in the unloading interface. 
Error is the error message for loading or unloading.

3. HOOK

Hotgo can add a callback function before and after loading and unloading hot patches. The function name is:

- HPBeforeLoad
- HPAfterLoad
- HPBeforeUnload
- HPAfterUnload

These four callback functions can be defined in the hot patch implementation file and called before and after the hot patch is loaded / unloaded, respectively. 
The function prototypes are all `func () error`.

## Example

The example code for the hot patch is located in the [example directory](./example/).

`main` directory is the main program.


```go
package main

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"hotgo"
)

var globalValue int = 100

func func1() {
	fmt.Printf("func1: %d\n", globalValue)
}

func func2() {
	fmt.Printf("func2: %d\n", globalValue)
}

func main() {
	ticker := time.NewTicker(time.Second)
	func2()
	go func() {
		for range ticker.C {
			func1()
		}
	}()

	httpServer := hotgo.HPHttpServer()
	l, _ := net.Listen("tcp", ":8080")
	http.Serve(l, httpServer)
}
```

The program calls `func1` every second, and prints the value of the global variable `globalValue` in the function `func1`.

The hot patch source files under the patch directory:


```go
package main

var Global *int

var Func2Ref func()

func NewFunc1() {
	println("New Func: ", *Global)
	*Global += 15
	Func2Ref()
}

func HPBeforeLoad() error {
	println("HPBeforeLoad called")
	return nil
}

func HPAfterLoad() error {
	println("HPAfterLoad called")
	return nil
}

func HPBeforeUnload() error {
	println("HPBeforeUnload called")
	return nil
}

func HPAfterUnload() error {
	println("HPAfterUnload called")
	return nil
}
```

Execute the script `compile_and_ run.sh` to compile and run.

Use curl to load hot patches through the http interface.


```bash
curl -d '{
    "hp_file": "./hp.so",    
    "replace_config": {
        "NewFunc1":"main.func1"
    },
    "ref_config": {
        "Global": "main.globalValue",
        "Func2Ref": "main.func2"
    }
}' http://127.0.0.1:8080/hp/v1/load
```

The API returns the ID of the hot patch, as follows:

```json
{"hp_id":"adc68ba7bb59e583a7ccd52d6d99a4c3686075dde3649ae323994bf394eb89c6","error":""}
```

Observe the behavior of the program, which has changed. 

Then use curl to unload the hot patch through the http interface.

```bash
curl -d '{
    "hp_id": "adc68ba7bb59e583a7ccd52d6d99a4c3686075dde3649ae323994bf394eb89c6"
}' http://127.0.0.1:8080/hp/v1/unload
```

The returned result is as follows:

```json
{"hp_id":"","error":""}
```

The program has restored its original execution logic.

## Points for attention

There are different ways to reference variables and functions in the original application. 
The function is directly defined as the same signature as in the original program, but the variable needs to be defined as a pointer type of the corresponding type in the original program. 
Refer to the definitions of `NewFunc1` and `Global` variables in example. 
This is designed because the function is read-only, so you can copy the value to the patch, but the global variable may need to be modified, so you need to copy the address of the variable (otherwise two variables).

## Known issues

1. Non-concurrent security
2. Since plugin does not support close, the unloaded hot patch cannot be removed from memory.
3. Plugin loading so with the same path will only load the first time, so the hot patch cannot be reloaded under the same path after modification.
