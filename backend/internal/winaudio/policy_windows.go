//go:build windows

// Package winaudio kapselt Windows-spezifische Audio-Funktionen, die miniaudio nicht abdeckt:
// das Setzen des Standard-(Kommunikations-)Geräts über das undokumentierte, aber seit Vista stabile
// COM-Interface IPolicyConfig (das nutzen auch SoundSwitch, EarTrumpet & Co.).
package winaudio

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
)

// Rollen (ERole)
const (
	RoleConsole        = 0
	RoleMultimedia     = 1
	RoleCommunications = 2
)

var (
	clsidPolicyConfig = ole.NewGUID("{294935CE-F637-4E7C-A41B-AB255460B862}") // CPolicyConfigClient
	iidPolicyConfig   = ole.NewGUID("{F8679F50-850A-41CF-9C72-430F290290C8}") // IPolicyConfig
)

// Reihenfolge der Methoden ist ABI-relevant.
type policyConfigVtbl struct {
	ole.IUnknownVtbl
	GetMixFormat          uintptr
	GetDeviceFormat       uintptr
	ResetDeviceFormat     uintptr
	SetDeviceFormat       uintptr
	GetProcessingPeriod   uintptr
	SetProcessingPeriod   uintptr
	GetShareMode          uintptr
	SetShareMode          uintptr
	GetPropertyValue      uintptr
	SetPropertyValue      uintptr
	SetDefaultEndpoint    uintptr
	SetEndpointVisibility uintptr
}

// SetDefaultEndpoint setzt das Gerät mit der WASAPI-Endpoint-ID für die gegebenen Rollen als Standard.
func SetDefaultEndpoint(deviceID string, roles ...int) error {
	if len(roles) == 0 {
		return fmt.Errorf("keine Rolle angegeben")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		// S_FALSE (bereits initialisiert) wird von go-ole als Fehler geliefert; weiter, falls das Apartment passt.
		if oe, ok := err.(*ole.OleError); !ok || (oe.Code() != 1 /* S_FALSE */ && oe.Code() != 0x80010106 /* RPC_E_CHANGED_MODE */) {
			return fmt.Errorf("CoInitializeEx: %w", err)
		}
	} else {
		defer ole.CoUninitialize()
	}

	unk, err := ole.CreateInstance(clsidPolicyConfig, iidPolicyConfig)
	if err != nil {
		return fmt.Errorf("IPolicyConfig nicht verfügbar: %w", err)
	}
	defer unk.Release()
	vtbl := (*policyConfigVtbl)(unsafe.Pointer(unk.RawVTable))

	id, err := syscall.UTF16PtrFromString(deviceID)
	if err != nil {
		return err
	}
	for _, role := range roles {
		hr, _, _ := syscall.SyscallN(vtbl.SetDefaultEndpoint, uintptr(unsafe.Pointer(unk)), uintptr(unsafe.Pointer(id)), uintptr(role))
		if hr != 0 {
			return fmt.Errorf("SetDefaultEndpoint(Rolle %d) fehlgeschlagen: HRESULT 0x%08X", role, uint32(hr))
		}
	}
	return nil
}
