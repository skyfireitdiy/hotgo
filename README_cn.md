# hotgo - Go 热补丁支持

[English](./README.md)

hotgo 是一个用于为应用程序提供热补丁功能的Go语言包。

## 快速入门

1. 使应用程序支持热补丁功能

在应用程序中启动热补丁http服务器：

```go
httpServer := hotgo.HPHttpServer()
l, _ := net.Listen("tcp", ":8080")
http.Serve(l, httpServer)
```

或者rpc服务器：

```go
rpcserver := hotgo.HPRpcServer()
rpcserver.HandleHTTP("/hp", "/hpdebug")
http.ListenAndServe(":8080", rpcserver)
```

为了防止编译器将符号优化掉，建议编译时增加`-gcflags="-N -l"` 参数。

2. 制作热补丁文件

热补丁文件其实就是go plugin支持的so文件，使用命令参数`--buildmode=plugin`就可以编译出来。

3. 使用http请求或者rpc调用加载热补丁

以下以http为例：

```bash
curl -d '{
    "hp_file": "./hp.so",    
    "replace_config": {
        "NewFunc":"main.oldFunc"
    },
    "ref_config": {
        "Data": "main.global",
        "NewHiddenFunc": "main.hiddenFunc"
    }
}' http://127.0.0.1:8080/hp/v1/load
```

## 接口说明

1. hotgo 提供以下函数接口

- func LoadHP(config Config) (hpID string, errret error)
  
  使用指定的配置加载热补丁，成功则返回热补丁ID，失败返回错误信息。

- func UnloadHP(hpID string) (errret error)

  卸载指定ID的热补丁，失败返回错误原因。

- func HPHttpServer() http.Handler

  创建一个热补丁HttpServer，用于使用http协议访问热补丁加载与卸载接口（此接口仅返回一个Handler，需要自行监听端口提供服务）。
  
- func HPRpcServer() *rpc.Server

  创建一个RpcServer，用于使用http协议访问热补丁加载与卸载接口。此接口与 HPHttpServer 相同，仅返回一个Server，需要自行监听提供服务。

- func GetHPInfo() []HPInfo

  获取当前已经加载的热补丁信息。

1. hotgo 提供以下公开的结构体

```go
type Config struct {
	HPFile        string            `json:"hp_file"`
	ReplaceConfig map[string]string `json:"replace_config"`
	RefConfig     map[string]string `json:"ref_config"`
}
```

加载热补丁需要的配置。

- HPFile：热补丁文件存放路径
- ReplaceConfig：替换映射表。映射的键为热补丁中的符号名称，值为正在运行程序中的需要被替换的符号名称
- RefConfig：引用映射表。映射的键为热补丁中的符号名称，值为正在运行程序中需要引用的 符号名称

```go
type LoadRequest Config
```

LoadRequest 是 Config 结构的一个别名，用于http或者rpc接口的参数使用。

```go
type UnloadRequest struct {
	HPID string `json:"hp_id"`
}
```

UnloadRequest 是热补丁卸载接口需要的参数类型。HPID 是由加载接口返回的值。

```go
type LoadResponse struct {
	HPID  string `json:"hp_id"`
	Error string `json:"error"`
}
```

http或者rpc加载热补丁接口的返回类型。HPID 是在热补丁加载接口中返回的热补丁ID，Error 是加载错误信息。

```go
type UnloadResponse struct {
	Error string `json:"error"`
}
```

http或者rpc卸载热补丁接口的返回类型，Error 是卸载错误信息。

```go
type InfoRequest struct{}
```

热补丁信息查询接口参数类型，主要为rpc提供参数类型，无实际意义。

```go
type InfoResponse struct {
	Info []HPInfo `json:"info"`
}
```

热补丁信息查询接口返回类型，Info 为热补丁信息切片。

```go
type HPInfo struct {
	HPID   string `json:"hp_id"`
	Config Config `json:"config"`
}
```

热补丁信息。

1. HOOK

hotgo 可以在加载与卸载热补丁前后增加回调函数，函数名称为：

- HPBeforeLoad
- HPAfterLoad
- HPBeforeUnload
- HPAfterUnload

这四个回调函数可以定义在热补丁实现文件中，分别在热补丁加载/卸载前后调用。函数原型均为`func() error`。

## 例子

热补丁的例子代码位于[example目录](./example/)。

`main`目录为主程序。

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

程序每隔一秒调用一次`func1`，`func1`函数中打印全局变量`globalValue`的值。

patch目录下为热补丁源文件：

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

执行脚本 `compile_and_run.sh` 编译运行。

使用 curl 通过 http 接口加载热补丁。

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

接口会返回热补丁的ID，如下：

```json
{"hp_id":"adc68ba7bb59e583a7ccd52d6d99a4c3686075dde3649ae323994bf394eb89c6","error":""}
```

观察程序行为，已经发生改变。

然后使用 curl 通过 http 接口查看当前加载的热补丁。

```bash
curl http://127.0.0.1:8080/hp/v1/info
```

返回结果如下：

```json
[{"hp_id":"adc68ba7bb59e583a7ccd52d6d99a4c3686075dde3649ae323994bf394eb89c6","config":{"hp_file":"./hp.so","replace_config":{"NewFunc1":"main.func1"},"ref_config":{"Func2Ref":"main.func2","Global":"main.globalValue"}}}]
```

然后使用 curl 通过 http 接口卸载热补丁。

```bash
curl -d '{
    "hp_id": "adc68ba7bb59e583a7ccd52d6d99a4c3686075dde3649ae323994bf394eb89c6"
}' http://127.0.0.1:8080/hp/v1/unload
```

返回结果如下：

```json
{"hp_id":"","error":""}
```

程序已恢复原来的执行逻辑。

## 注意事项

对于引用原应用程序中的变量与函数的方式是不同的。函数直接定义为与原程序中相同的签名，但是变量需要定义为与原程序中对应类型的指针类型。参照example中`NewFunc1`与`Global`变量的定义。之所以这么设计，是因为函数是只读的，因此可以将值复制到补丁中，但是全局变量可能需要修改，因此需要复制变量的地址（否则就是两个变量）。

## 已知问题

1. 非并发安全
2. 由于plugin不支持关闭，所以无法将卸载后的热补丁从内存中清除
3. plugin加载相同路径的so仅会加载第一次，因此无法将热补丁修改后在相同路径下重新加载
