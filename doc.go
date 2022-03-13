// Package Hotgo is a component that dynamically patches application functions without exiting the GO application (hot patches)
//
// As quick start, you can use the following code to support hot patching:
//	httpServer := hotgo.HPHttpServer()
//	l, _ := net.Listen("tcp", ":8080")
//	http.Serve(l, httpServer)
// You can then hot patch running applications over HTTP:
//	curl -d '{
//    "hp_file": "./hp.so",
//    "replace_config": {
//        "NewFunc":"main.oldFunc"
//    },
//    "ref_config": {
//        "Data": "main.global",
//        "NewHiddenFunc": "main.hiddenFunc"
//    }
//	}' http://127.0.0.1:8080/hp/v1/load
//	// Output:
//	// {"hp_id":"2641aa7612357c7c247c09f7a68f4d42b209a9258121653fa4854c2904c787f2","error":""}
//	// * Connection #0 to host 127.0.0.1 left intact
package hotgo
