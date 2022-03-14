package hotgo

import (
	"crypto/sha256"
	"debug/elf"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"plugin"
	"reflect"
	"sort"
	"syscall"
	"unsafe"
)

type replaceData struct {
	addr    uintptr
	oldCode []byte
}

type patchInfo struct {
	config  Config
	plugin_ *plugin.Plugin
}

type patchData struct {
	patchInfo   patchInfo
	replaceData []replaceData
	refData     []*uint64
}

type HPRpcService struct{}

const (
	symTypeVariable = 17
	symTypeFunction = 18
)

const (
	beforeLoadFuncName   = "HPBeforeLoad"
	afterLoadFuncName    = "HPAfterLoad"
	beforeUnloadFuncName = "HPBeforeUnload"
	afterUnloadFuncName  = "HPAfterUnload"
)

var globalHPData = map[string]patchData{}

func findSym(syms []elf.Symbol, name string) (elf.Symbol, bool) {
	left, right := 0, len(syms)
	for left < right {
		mid := (left + right) / 2
		if syms[mid].Name == name {
			return syms[mid], true
		}
		if syms[mid].Name < name {
			left = mid + 1
		} else {
			right = mid
		}
	}
	return elf.Symbol{}, false
}

func fillRef(esym elf.Symbol, psym plugin.Symbol) *uint64 {
	v := esym.Value
	newAddr := (*uint64)(unsafe.Pointer(uintptr(((*fakeInterface)(unsafe.Pointer(&psym))).value)))

	if esym.Info == symTypeVariable {
		*newAddr = v
	} else if esym.Info == symTypeFunction {
		*newAddr = uint64(uintptr(unsafe.Pointer(&v)))
	} else {
		panic("unknown ref type")
	}
	return &v
}

type fakeInterface struct {
	type_ unsafe.Pointer
	value unsafe.Pointer
}

func mprotect(addr uintptr, len uintptr, prop int) {
	pageSize := syscall.Getpagesize()
	pageStart := addr & ^(uintptr(pageSize - 1))
	for p := pageStart; p < addr+len; p += uintptr(pageSize) {
		page := memoryAccess(p, pageSize)
		err := syscall.Mprotect(page, prop)
		if err != nil {
			panic(err)
		}
	}
}

func memoryAccess(addr uintptr, len int) []byte {
	return *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
		Data: addr,
		Len:  len,
		Cap:  len,
	}))
}

func replaceSym(esym elf.Symbol, psym plugin.Symbol) replaceData {
	addr := esym.Value
	newAddr := uint64(uintptr(((*fakeInterface)(unsafe.Pointer(&psym))).value))
	return replaceData{
		addr:    uintptr(addr),
		oldCode: setFuncJumpToAddr(addr, newAddr),
	}
}

func writeMemory(addr uintptr, code []byte) []byte {
	oldMem := memoryAccess(uintptr(addr), len(code))
	oldCode := make([]byte, len(code))
	copy(oldCode, oldMem)
	mprotect(uintptr(addr), uintptr(len(code)), syscall.PROT_READ|syscall.PROT_WRITE|syscall.PROT_EXEC)
	copy(oldMem, code[:])
	mprotect(uintptr(addr), uintptr(len(code)), syscall.PROT_READ|syscall.PROT_EXEC)
	return oldCode
}

func getHPID(patchFile string) (string, error) {
	file, err := os.Open(patchFile)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func getElfSyms() ([]elf.Symbol, error) {
	elfFile, err := elf.Open(os.Args[0])
	if err != nil {
		return nil, err
	}
	defer func() {
		elfFile.Close()
	}()
	syms, err := elfFile.Symbols()
	if err != nil {
		return nil, err
	}
	sort.Slice(syms, func(i, j int) bool {
		return syms[i].Name < syms[j].Name
	})
	return syms, nil
}

func runHookFunc(plugin_ *plugin.Plugin, functionName string) error {
	// get interface from plugin
	hook, err := plugin_.Lookup(functionName)
	if err != nil {
		// no BeforeLoad function, not an error
		return nil
	}
	hookFunc, ok := hook.(func() error)
	if !ok {
		return fmt.Errorf("%s is not a function  or function signature is not `func() error`", functionName)
	}
	return hookFunc()
}

func resolveRefConfig(config Config, syms []elf.Symbol, plugin_ *plugin.Plugin) ([]*uint64, error) {
	tmpRefData := []*uint64{}
	for hpSym, refSym := range config.RefConfig {
		esym, ok := findSym(syms, refSym)
		if !ok {
			return nil, fmt.Errorf("symbol %s not found", refSym)
		}
		psym, err := plugin_.Lookup(hpSym)
		if err != nil {
			return nil, err
		}
		tmpRefData = append(tmpRefData, fillRef(esym, psym))
	}
	return tmpRefData, nil
}

func resolveReplaceConfig(config Config, syms []elf.Symbol, plugin_ *plugin.Plugin) ([]replaceData, error) {
	replaceData := []replaceData{}
	for hpSym, oldSym := range config.ReplaceConfig {
		esym, ok := findSym(syms, oldSym)
		if !ok {
			return nil, fmt.Errorf("symbol %s not found", oldSym)
		}
		psym, err := plugin_.Lookup(hpSym)
		if err != nil {
			return nil, err
		}
		replaceData = append(replaceData, replaceSym(esym, psym))
	}
	return replaceData, nil
}

func loadHPHttpHandleFunc(res http.ResponseWriter, req *http.Request) {
	// Get post data as JSON
	var config LoadRequest

	if err := json.NewDecoder(req.Body).Decode(&config); err != nil {
		res.WriteHeader(http.StatusBadRequest)
		// write response body as JSON to http response
		json.NewEncoder(res).Encode(LoadResponse{Error: err.Error()})
		return
	}

	// Load patch
	if hpID, err := LoadHP(Config(config)); err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		// write response body as JSON to http response
		json.NewEncoder(res).Encode(LoadResponse{Error: err.Error()})
		return
	} else {
		res.WriteHeader(http.StatusOK)
		// write response body as JSON to http response
		json.NewEncoder(res).Encode(LoadResponse{HPID: hpID})
	}
}

func unloadHPHttpHandleFunc(res http.ResponseWriter, req *http.Request) {
	// Get UnloadHPRequest from request as JSON
	var request UnloadRequest

	if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
		res.WriteHeader(http.StatusBadRequest)
		// write response body as JSON to http response
		json.NewEncoder(res).Encode(LoadResponse{Error: err.Error()})
		return
	}

	// Unload patch
	if err := UnloadHP(request.HPID); err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		// write response body as JSON to http response
		json.NewEncoder(res).Encode(LoadResponse{Error: err.Error()})
		return
	} else {
		res.WriteHeader(http.StatusOK)
		// write response body as JSON to http response
		json.NewEncoder(res).Encode(LoadResponse{})
	}
}

func infoHttpHandleFunc(res http.ResponseWriter, req *http.Request) {
	info := GetHPInfo()
	res.WriteHeader(http.StatusOK)
	// write response body as JSON to http response
	json.NewEncoder(res).Encode(info)
}

func (s *HPRpcService) LoadHP(params Config, result *LoadResponse) error {
	// Load patch
	if hpID, err := LoadHP(params); err != nil {
		return err
	} else {
		result.HPID = hpID
	}
	return nil
}

func (s *HPRpcService) UnloadHP(params UnloadRequest, result *UnloadResponse) error {
	// Unload patch
	if err := UnloadHP(params.HPID); err != nil {
		result.Error = err.Error()
	}
	return nil
}

func (s *HPRpcService) Info(_ InfoRequest, result *InfoResponse) error {
	result.Info = GetHPInfo()
	return nil
}

func setFuncJumpToAddr(addr uint64, newAddr uint64) []byte {
	code := makeMachineCode(newAddr)
	oldCode := writeMemory(uintptr(addr), code)
	return oldCode
}
