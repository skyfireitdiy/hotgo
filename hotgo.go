package hotgo

import (
	"crypto/sha256"
	"debug/elf"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"plugin"
	"reflect"
	"sort"
	"syscall"
	"unsafe"
)

type ReplaceConfig struct {
	OldSym string
	NewSym string
}

type RefConfig struct {
	RefSym   string
	PatchSym string
}

type Config struct {
	ReplaceConfigs []ReplaceConfig
	RefConfigs     []RefConfig
}

type replaceData struct {
	addr    uintptr
	oldCode []byte
}

type patchInfo struct {
	filename string
	config   Config
	plugin_  *plugin.Plugin
}

type patchData struct {
	patchInfo   patchInfo
	replaceData []replaceData
	refData     []*uint64
}

var globalPatchData = map[string]patchData{}

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
	println(esym.Info)
	if esym.Info == 17 {
		*newAddr = v
	} else if esym.Info == 18 {
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
	code := makeMachineCode(newAddr)
	oldMem := memoryAccess(uintptr(addr), len(code))
	oldCode := make([]byte, len(code))
	copy(oldCode, oldMem)
	mprotect(uintptr(addr), uintptr(len(code)), syscall.PROT_READ|syscall.PROT_WRITE|syscall.PROT_EXEC)
	copy(oldMem, code[:])
	mprotect(uintptr(addr), uintptr(len(code)), syscall.PROT_READ|syscall.PROT_EXEC)
	return replaceData{
		addr:    uintptr(addr),
		oldCode: oldCode,
	}
}

func getHpID(patchFile string) (string, error) {
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

func Patch(patchFile string, config Config) (string, error) {
	elfFile, err := elf.Open(os.Args[0])
	if err != nil {
		return "", err
	}
	defer func() {
		elfFile.Close()
	}()
	syms, err := elfFile.Symbols()
	if err != nil {
		return "", err
	}
	sort.Slice(syms, func(i, j int) bool {
		return syms[i].Name < syms[j].Name
	})
	plugin_, err := plugin.Open(patchFile)
	if err != nil {
		return "", err
	}
	hpID, err := getHpID(patchFile)
	tmpPatchInfo := patchInfo{
		filename: patchFile,
		config:   config,
		plugin_:  plugin_,
	}
	replaceData := []replaceData{}
	if err != nil {
		return "", err
	}
	tmpRefData := []*uint64{}
	for _, ref := range config.RefConfigs {
		esym, ok := findSym(syms, ref.RefSym)
		if !ok {
			return "", fmt.Errorf("symbol %s not found", ref.RefSym)
		}
		psym, err := plugin_.Lookup(ref.PatchSym)
		if err != nil {
			return "", err
		}
		tmpRefData = append(tmpRefData, fillRef(esym, psym))
	}
	for _, rep := range config.ReplaceConfigs {
		esym, ok := findSym(syms, rep.OldSym)
		if !ok {
			return "", fmt.Errorf("symbol %s not found", rep.OldSym)
		}
		psym, err := plugin_.Lookup(rep.NewSym)
		if err != nil {
			return "", err
		}
		replaceData = append(replaceData, replaceSym(esym, psym))
	}
	globalPatchData[hpID] = patchData{
		patchInfo:   tmpPatchInfo,
		replaceData: replaceData,
		refData:     tmpRefData,
	}
	return "", nil
}
