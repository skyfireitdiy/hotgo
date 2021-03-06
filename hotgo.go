package hotgo

import (
	"fmt"
	"net/http"
	"net/rpc"
	"plugin"
)

// Hot Patch Configuration
//
// HPFile: the path of the hot patch file
//
// ReplaceConfig: Replace the function mapping table with keys for function names in the hot patch file and values for symbol names in the running application
//
// RefConfig: Reference mapping tables. In the hot patch file, symbols (global variables or functions) in the running program are referenced. The keys are the names of functions and variables in the hot patch, and the values are symbols in the running program
type Config struct {
	HPFile        string            `json:"hp_file"`
	ReplaceConfig map[string]string `json:"replace_config"`
	RefConfig     map[string]string `json:"ref_config"`
}

// Request object for loading hot patches, used for http or rpc interface requests
type LoadRequest Config

// Request object for unloading hot patches, for http or rpc interface requests
type UnloadRequest struct {
	HPID string `json:"hp_id"`
}

// Hot patch load response object
type LoadResponse struct {
	HPID  string `json:"hp_id"`
	Error string `json:"error"`
}

// Hot patch unload response object
type UnloadResponse struct {
	Error string `json:"error"`
}

// Hot Patch Information
type HPInfo struct {
	HPID   string `json:"hp_id"`
	Config Config `json:"config"`
}

// Parameter type of hot patch information
type InfoRequest struct{}

// Return type of hot patch information
type InfoResponse struct {
	Info []HPInfo `json:"info"`
}

// Uninstall hot patch. Parameter is hot patch ID.
func UnloadHP(hpID string) (errret error) {
	defer func() {
		if err := recover(); err != nil {
			errret = fmt.Errorf("%v", err)
		}
	}()

	if _, ok := globalHPData[hpID]; !ok {
		return fmt.Errorf("patch %s not loaded", hpID)
	}

	patchData := globalHPData[hpID]

	// run BeforeUnload
	err := runHookFunc(patchData.patchInfo.plugin_, beforeUnloadFuncName)
	if err != nil {
		return err
	}

	// recover replaceData
	for _, replaceData := range patchData.replaceData {
		writeMemory(replaceData.addr, replaceData.oldCode)
	}

	// run AfterUnload
	err = runHookFunc(patchData.patchInfo.plugin_, afterUnloadFuncName)
	if err != nil {
		return err
	}

	// delete record
	delete(globalHPData, hpID)

	return nil
}

// Get the current hot patch loading information
func GetHPInfo() []HPInfo {
	var info []HPInfo
	for hpID, hpData := range globalHPData {
		info = append(info, HPInfo{
			HPID:   hpID,
			Config: hpData.patchInfo.config,
		})
	}
	return info
}

// Load hot patch. Parameter is hot patch configuration. If loaded successfully, hot patch ID will be returned, which is used to unload or query hot patch information.
func LoadHP(config Config) (hpID string, errret error) {

	defer func() {
		if err := recover(); err != nil {
			errret = fmt.Errorf("%v", err)
		}
	}()

	hpID, err := getHPID(config.HPFile)
	if err != nil {
		return "", err
	}

	// if already loaded
	if _, ok := globalHPData[hpID]; ok {
		return hpID, fmt.Errorf("patch %s already loaded", hpID)
	}

	syms, err := getElfSyms()
	if err != nil {
		return "", err
	}

	plugin_, err := plugin.Open(config.HPFile)
	if err != nil {
		return "", err
	}

	// run BeforeLoad
	err = runHookFunc(plugin_, beforeLoadFuncName)
	if err != nil {
		return "", err
	}

	tmpRefData, err := resolveRefConfig(config, syms, plugin_)
	if err != nil {
		return "", err
	}

	replaceData, err := resolveReplaceConfig(config, syms, plugin_)
	if err != nil {
		return "", err
	}

	globalHPData[hpID] = patchData{
		patchInfo: patchInfo{
			config:  config,
			plugin_: plugin_,
		},
		replaceData: replaceData,
		refData:     tmpRefData,
	}

	// run AfterLoad
	err = runHookFunc(plugin_, afterLoadFuncName)
	if err != nil {
		UnloadHP(hpID)
		return "", err
	}

	return hpID, nil
}

// Create a hot patch http server
func HPHttpServer() http.Handler {
	server := http.NewServeMux()
	server.HandleFunc("/hp/v1/load", loadHPHttpHandleFunc)
	server.HandleFunc("/hp/v1/unload", unloadHPHttpHandleFunc)
	server.HandleFunc("/hp/v1/info", infoHttpHandleFunc)
	return server
}

// Create a hot patch rpc server
func HPRpcServer() *rpc.Server {
	server := rpc.NewServer()
	server.Register(&HPRpcService{})
	return server
}
