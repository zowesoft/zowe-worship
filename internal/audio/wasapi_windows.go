//go:build windows

package audio

import (
	"encoding/binary"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// WASAPI in shared mode, driven entirely from Go: no cgo, no capture library.
// Every interface is reached through its vtable with the this pointer first,
// exactly as the x64 platform lays it out, so the whole audio path stays
// inside the one CGO_ENABLED=0 binary the app ships as.
var (
	ole32 = windows.NewLazySystemDLL("ole32.dll")

	procCoInitializeEx   = ole32.NewProc("CoInitializeEx")
	procCoUninitialize   = ole32.NewProc("CoUninitialize")
	procCoCreateInstance = ole32.NewProc("CoCreateInstance")
	procCoTaskMemFree    = ole32.NewProc("CoTaskMemFree")
	procPropVariantClear = ole32.NewProc("PropVariantClear")
)

var (
	clsidMMDeviceEnumerator = windows.GUID{Data1: 0xbcde0395, Data2: 0xe52f, Data3: 0x467c, Data4: [8]byte{0x8e, 0x3d, 0xc4, 0x57, 0x92, 0x91, 0x69, 0x2e}}
	iidIMMDeviceEnumerator  = windows.GUID{Data1: 0xa95664d2, Data2: 0x9614, Data3: 0x4f35, Data4: [8]byte{0xa7, 0x46, 0xde, 0x8d, 0xb6, 0x36, 0x17, 0xe6}}
	iidIAudioClient         = windows.GUID{Data1: 0x1cb9ad4c, Data2: 0xdbfa, Data3: 0x4c32, Data4: [8]byte{0xb1, 0x78, 0xc2, 0xf5, 0x68, 0xa7, 0x03, 0xb2}}
	iidIAudioCaptureClient  = windows.GUID{Data1: 0xc8adbd64, Data2: 0xe71e, Data3: 0x48a0, Data4: [8]byte{0xa4, 0xde, 0x18, 0x5c, 0x39, 0x5c, 0xd3, 0x17}}

	// PKEY_Device_FriendlyName, the name the operator recognises in the menu.
	pkeyDeviceFriendlyName = propertyKey{
		fmtid: windows.GUID{Data1: 0xa45c254e, Data2: 0xdf1c, Data3: 0x4efd, Data4: [8]byte{0x80, 0x20, 0x67, 0xd1, 0x46, 0xa8, 0x50, 0xe0}},
		pid:   14,
	}
)

// Method indices into each interface's vtable: IUnknown occupies 0..2 in
// every one of them.
const (
	methodEnumAudioEndpoints      = 3
	methodGetDefaultAudioEndpoint = 4
	methodGetDevice               = 5

	methodGetCount = 3
	methodItem     = 4

	methodActivate          = 3
	methodOpenPropertyStore = 4
	methodGetID             = 5

	methodGetValue = 5

	methodInitialize     = 3
	methodGetMixFormat   = 8
	methodStart          = 10
	methodStop           = 11
	methodSetEventHandle = 13
	methodGetService     = 14

	methodGetBuffer         = 3
	methodReleaseBuffer     = 4
	methodGetNextPacketSize = 5
)

const (
	coInitMultithreaded = 0x0
	rpcEChangedMode     = 0x80010106

	clsCtxAll = 0x17

	eCapture          = 1
	eConsole          = 0
	deviceStateActive = 0x00000001
	stgmRead          = 0x0

	vtBSTR   = 8
	vtLPWSTR = 31

	audclntSharemodeShared          = 0
	audclntStreamflagsEventCallback = 0x00040000
	audclntBufferflagsSilent        = 0x2

	waveFormatPcm        = 0x0001
	waveFormatIeeeFloat  = 0x0003
	waveFormatExtensible = 0xFFFE

	// captureBufferHNS is the engine buffer - 200 ms in 100 ns units - so a
	// late loop reads the next packet instead of losing audio to it.
	captureBufferHNS = 2_000_000

	// eventWaitMS bounds the wait for a packet signal so a closed stream is
	// noticed even if the device stops signalling altogether.
	eventWaitMS = 500

	// pollWaitMS is the fallback cadence for engines that refuse
	// event-driven capture.
	pollWaitMS = 25
)

// comError is a failed HRESULT, named for the call that returned it so a
// capture failure in the panel says which step refused.
type comError struct {
	op string
	hr uint32
}

func (e *comError) Error() string {
	return fmt.Sprintf("%s failed (0x%08X)", e.op, e.hr)
}

type propertyKey struct {
	fmtid windows.GUID
	pid   uint32
}

// propVariant is the x64 layout: type and reserved words at the head, then
// the value union. The union is sixteen bytes wide (DECIMAL is its largest
// member), so a PROPVARIANT is twenty-four bytes - the property store writes
// all of it, and a shorter struct would be overrun.
type propVariant struct {
	vt       uint16
	reserved [6]byte
	value    [16]byte
}

// pointer reads the union as the pointer it holds for the string types.
func (v *propVariant) pointer() unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&v.value[0]))
}

type waveFormatEx struct {
	FormatTag      uint16
	Channels       uint16
	SampleRate     uint32
	AvgBytesPerSec uint32
	BlockAlign     uint16
	BitsPerSample  uint16
	Size           uint16
}

// comInvoke runs method `index` of the COM interface at obj, this pointer
// first as x64 calling expects, and returns the raw result.
func comInvoke(obj unsafe.Pointer, index int, args ...uintptr) uintptr {
	if obj == nil {
		return 0
	}
	vtable := *(**uintptr)(obj)
	procedure := *(*uintptr)(unsafe.Add(unsafe.Pointer(vtable), uintptr(index)*unsafe.Sizeof(uintptr(0))))
	argv := make([]uintptr, 0, len(args)+1)
	argv = append(argv, uintptr(obj))
	argv = append(argv, args...)
	result, _, _ := syscall.SyscallN(procedure, argv...)
	return result
}

// comCall runs it and reports a negative HRESULT. Methods whose return is not
// an HRESULT (AddRef, Release) go through comInvoke instead.
func comCall(obj unsafe.Pointer, index int, op string, args ...uintptr) error {
	if result := comInvoke(obj, index, args...); int32(result) < 0 {
		return &comError{op: op, hr: uint32(result)}
	}
	return nil
}

func release(obj unsafe.Pointer) {
	if obj == nil {
		return
	}
	comInvoke(obj, 2)
}

// comInit puts the calling goroutine's thread in the process's multithreaded
// apartment. An apartment belongs to an OS thread and Go moves goroutines
// between threads, so the thread is pinned for as long as COM is in use; the
// returned func gives back both the apartment and the pin and must run before
// the goroutine returns.
func comInit() (func(), error) {
	runtime.LockOSThread()

	uninit := true
	hr, _, _ := procCoInitializeEx.Call(0, coInitMultithreaded)
	switch uint32(hr) {
	case 0, 1: // S_OK, S_FALSE
	case rpcEChangedMode:
		// The thread is already in another apartment: COM still answers here,
		// but this call took no reference, so there is nothing to uninitialise.
		uninit = false
	default:
		runtime.UnlockOSThread()
		return nil, &comError{op: "ole32.CoInitializeEx", hr: uint32(hr)}
	}

	return func() {
		if uninit {
			procCoUninitialize.Call()
		}
		runtime.UnlockOSThread()
	}, nil
}

// enumerateInputs lists the active capture devices, default marked. It opens
// and closes COM entirely within this call, so it is safe from a binding.
func enumerateInputs() ([]Device, error) {
	uninit, err := comInit()
	if err != nil {
		return nil, err
	}
	defer uninit()

	enumerator, err := newEnumerator()
	if err != nil {
		return nil, err
	}
	defer release(enumerator)

	var collection unsafe.Pointer
	if err := comCall(enumerator, methodEnumAudioEndpoints, "IMMDeviceEnumerator.EnumAudioEndpoints",
		eCapture, deviceStateActive, uintptr(unsafe.Pointer(&collection))); err != nil {
		return nil, err
	}
	defer release(collection)

	var count uint32
	if err := comCall(collection, methodGetCount, "IMMDeviceCollection.GetCount",
		uintptr(unsafe.Pointer(&count))); err != nil {
		return nil, err
	}

	defaultID := defaultInputID(enumerator)

	devices := make([]Device, 0, count)
	for index := uint32(0); index < count; index++ {
		var dev unsafe.Pointer
		if err := comCall(collection, methodItem, "IMMDeviceCollection.Item",
			uintptr(index), uintptr(unsafe.Pointer(&dev))); err != nil {
			return nil, err
		}
		id, label, idErr := deviceIdentity(dev)
		release(dev)
		if idErr != nil {
			// A device that will not name itself is not one the operator can
			// choose by name, so it is left out rather than listed as blank.
			continue
		}
		devices = append(devices, Device{DeviceID: id, Label: label, Default: id == defaultID})
	}
	return devices, nil
}

func newEnumerator() (unsafe.Pointer, error) {
	var obj unsafe.Pointer
	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidMMDeviceEnumerator)),
		0,
		clsCtxAll,
		uintptr(unsafe.Pointer(&iidIMMDeviceEnumerator)),
		uintptr(unsafe.Pointer(&obj)),
	)
	if int32(hr) < 0 {
		return nil, &comError{op: "ole32.CoCreateInstance(MMDeviceEnumerator)", hr: uint32(hr)}
	}
	return obj, nil
}

func defaultInputID(enumerator unsafe.Pointer) string {
	var dev unsafe.Pointer
	if err := comCall(enumerator, methodGetDefaultAudioEndpoint, "IMMDeviceEnumerator.GetDefaultAudioEndpoint",
		eCapture, eConsole, uintptr(unsafe.Pointer(&dev))); err != nil {
		return ""
	}
	defer release(dev)

	id, _, err := deviceIdentity(dev)
	if err != nil {
		return ""
	}
	return id
}

// openInput resolves the device to open: the default endpoint for an empty
// id, the named one otherwise.
func openInput(enumerator unsafe.Pointer, deviceID string) (unsafe.Pointer, error) {
	if deviceID == "" {
		var dev unsafe.Pointer
		if err := comCall(enumerator, methodGetDefaultAudioEndpoint, "IMMDeviceEnumerator.GetDefaultAudioEndpoint",
			eCapture, eConsole, uintptr(unsafe.Pointer(&dev))); err != nil {
			return nil, fmt.Errorf("no default microphone: %w", err)
		}
		return dev, nil
	}

	id, err := windows.UTF16PtrFromString(deviceID)
	if err != nil {
		return nil, fmt.Errorf("input device id %q: %w", deviceID, err)
	}
	var dev unsafe.Pointer
	callErr := comCall(enumerator, methodGetDevice, "IMMDeviceEnumerator.GetDevice",
		uintptr(unsafe.Pointer(id)), uintptr(unsafe.Pointer(&dev)))
	runtime.KeepAlive(id)
	if callErr != nil {
		return nil, fmt.Errorf("input device %q is not available: %w", deviceID, callErr)
	}
	return dev, nil
}

// deviceIdentity reads the id the bridge hands back and the friendly name the
// operator sees. The id is what has to survive between calls, so it is the one
// thing that cannot be tolerated missing.
func deviceIdentity(dev unsafe.Pointer) (id, label string, err error) {
	var idPtr *uint16
	if err := comCall(dev, methodGetID, "IMMDevice.GetId", uintptr(unsafe.Pointer(&idPtr))); err != nil {
		return "", "", err
	}
	defer procCoTaskMemFree.Call(uintptr(unsafe.Pointer(idPtr)))

	id = utf16Ptr(idPtr)
	if id == "" {
		return "", "", fmt.Errorf("IMMDevice.GetId returned an empty id")
	}
	label = deviceLabel(dev)
	if label == "" {
		label = "Microphone"
	}
	return id, label, nil
}

func deviceLabel(dev unsafe.Pointer) string {
	var store unsafe.Pointer
	if err := comCall(dev, methodOpenPropertyStore, "IMMDevice.OpenPropertyStore",
		stgmRead, uintptr(unsafe.Pointer(&store))); err != nil {
		return ""
	}
	defer release(store)

	var value propVariant
	if err := comCall(store, methodGetValue, "IPropertyStore.GetValue",
		uintptr(unsafe.Pointer(&pkeyDeviceFriendlyName)), uintptr(unsafe.Pointer(&value))); err != nil {
		return ""
	}
	defer procPropVariantClear.Call(uintptr(unsafe.Pointer(&value)))

	if (value.vt == vtLPWSTR || value.vt == vtBSTR) && value.pointer() != nil {
		return utf16Ptr((*uint16)(value.pointer()))
	}
	return ""
}

func utf16Ptr(ptr *uint16) string {
	if ptr == nil {
		return ""
	}
	length := 0
	for *(*uint16)(unsafe.Add(unsafe.Pointer(ptr), uintptr(length)*2)) != 0 {
		length++
	}
	if length == 0 {
		return ""
	}
	return windows.UTF16ToString(unsafe.Slice(ptr, length))
}

// openDevice opens the input and pumps it on one goroutine that owns the COM
// objects for the stream's whole life: an apartment is a per-thread property,
// so a stream may not be created on one thread and driven from another.
// The handle it returns is ready; anything that goes wrong before that comes
// back as the error instead.
func openDevice(deviceID string, sink sink) (stream, Device, error) {
	type opened struct {
		device Device
		err    error
	}

	ready := make(chan opened, 1)
	handle := &wasapiStream{closed: make(chan struct{}), done: make(chan struct{})}

	go func() {
		defer close(handle.done)

		uninit, err := comInit()
		if err != nil {
			ready <- opened{err: err}
			return
		}
		defer uninit()

		engine, device, err := acquire(deviceID)
		if err != nil {
			ready <- opened{err: err}
			return
		}

		ready <- opened{device: device}
		if err := engine.run(handle.closed, sink); err != nil {
			sink.fail(err)
		}
		engine.release()
	}()

	result := <-ready
	if result.err != nil {
		return nil, Device{}, result.err
	}
	return handle, result.device, nil
}

// acquire resolves the device, takes it to a running capture and reports what
// was actually opened.
func acquire(deviceID string) (*wasapiEngine, Device, error) {
	enumerator, err := newEnumerator()
	if err != nil {
		return nil, Device{}, err
	}
	defer release(enumerator)

	dev, err := openInput(enumerator, deviceID)
	if err != nil {
		return nil, Device{}, err
	}
	defer release(dev)

	device, err := describe(dev, defaultInputID(enumerator))
	if err != nil {
		return nil, Device{}, err
	}

	engine, err := newEngine(dev)
	if err != nil {
		return nil, Device{}, err
	}
	return engine, device, nil
}

func describe(dev unsafe.Pointer, defaultID string) (Device, error) {
	id, label, err := deviceIdentity(dev)
	if err != nil {
		return Device{}, err
	}
	return Device{DeviceID: id, Label: label, Default: defaultID != "" && id == defaultID}, nil
}

// wasapiStream is the handle Service keeps: closing it signals the loop and
// waits for the goroutine to let go of its sink, so nothing pushes afterwards.
type wasapiStream struct {
	closed chan struct{}
	done   chan struct{}
	once   sync.Once
}

func (s *wasapiStream) close() error {
	s.once.Do(func() { close(s.closed) })
	<-s.done
	return nil
}

// newEngine prefers event-driven capture: the engine then signals when a
// packet is ready and the loop never polls the device, which is what the PRD's
// "do not poll hardware continuously" asks for. An engine that refuses that
// mode for capture gets the same stream opened without the flag, and the loop
// waits on a timeout instead.
func newEngine(dev unsafe.Pointer) (*wasapiEngine, error) {
	engine, eventErr := buildEngine(dev, true)
	if eventErr == nil {
		return engine, nil
	}
	engine, err := buildEngine(dev, false)
	if err != nil {
		return nil, fmt.Errorf("%w (event-driven capture: %v)", err, eventErr)
	}
	return engine, nil
}

// wasapiEngine is one live capture, owned by the goroutine that created it.
type wasapiEngine struct {
	client    unsafe.Pointer
	capture   unsafe.Pointer
	format    []byte // the engine's own mix format, verbatim for Initialize
	channels  int
	kind      sampleKind
	resampler *resampler
	event     windows.Handle // 0 when the engine refused event mode
	started   bool
}

// buildEngine activates a fresh IAudioClient on dev and takes it to Start. A
// fresh client per attempt matters: IAudioClient::Initialize may only be
// called once on a client, so a failed event-mode attempt cannot be retried on
// the same one.
func buildEngine(dev unsafe.Pointer, eventDriven bool) (*wasapiEngine, error) {
	var client unsafe.Pointer
	if err := comCall(dev, methodActivate, "IMMDevice.Activate",
		uintptr(unsafe.Pointer(&iidIAudioClient)), clsCtxAll, 0, uintptr(unsafe.Pointer(&client))); err != nil {
		return nil, err
	}

	format, err := readMixFormat(client)
	if err != nil {
		release(client)
		return nil, err
	}
	mix, err := parseMix(format)
	if err != nil {
		release(client)
		return nil, err
	}

	engine := &wasapiEngine{
		client:    client,
		format:    format,
		channels:  mix.channels,
		kind:      mix.kind,
		resampler: newResampler(mix.rate),
	}

	var flags uint32
	if eventDriven {
		event, evErr := windows.CreateEvent(nil, 0, 0, nil)
		if evErr != nil {
			engine.release()
			return nil, fmt.Errorf("create capture event: %w", evErr)
		}
		engine.event = event
		flags = audclntStreamflagsEventCallback
	}

	err = comCall(client, methodInitialize, "IAudioClient.Initialize",
		audclntSharemodeShared, uintptr(flags), uintptr(captureBufferHNS), 0,
		uintptr(unsafe.Pointer(&format[0])), 0)
	runtime.KeepAlive(format)
	if err != nil {
		engine.release()
		return nil, err
	}

	var capture unsafe.Pointer
	if err := comCall(client, methodGetService, "IAudioClient.GetService",
		uintptr(unsafe.Pointer(&iidIAudioCaptureClient)), uintptr(unsafe.Pointer(&capture))); err != nil {
		engine.release()
		return nil, err
	}
	engine.capture = capture

	if eventDriven {
		if err := comCall(client, methodSetEventHandle, "IAudioClient.SetEventHandle",
			uintptr(engine.event)); err != nil {
			engine.release()
			return nil, err
		}
	}
	if err := comCall(client, methodStart, "IAudioClient.Start"); err != nil {
		engine.release()
		return nil, err
	}
	engine.started = true
	return engine, nil
}

// release drops the COM objects. Stop can legitimately fail when the device
// has already gone away; the point is to let go, not to be told twice.
func (e *wasapiEngine) release() {
	if e.started {
		_ = comCall(e.client, methodStop, "IAudioClient.Stop")
		e.started = false
	}
	if e.capture != nil {
		release(e.capture)
		e.capture = nil
	}
	if e.client != nil {
		release(e.client)
		e.client = nil
	}
	if e.event != 0 {
		_ = windows.CloseHandle(e.event)
		e.event = 0
	}
}

// run pumps the device until the stream closes, and reports the error that
// ended it if it ended badly - a removed device does exactly that, and it is
// what audio.input.health_changed's error state is built from.
func (e *wasapiEngine) run(closed <-chan struct{}, sink sink) error {
	for {
		select {
		case <-closed:
			return nil
		default:
		}

		if err := e.drain(sink); err != nil {
			return err
		}

		if e.event != 0 {
			// The engine signals when a packet is ready; the timeout only
			// bounds how long a close takes to be noticed.
			if _, err := windows.WaitForSingleObject(e.event, eventWaitMS); err != nil {
				return fmt.Errorf("wait for capture event: %w", err)
			}
			continue
		}
		select {
		case <-closed:
			return nil
		case <-time.After(pollWaitMS * time.Millisecond):
		}
	}
}

// drain hands over every packet the engine has right now. Each packet is
// converted before its buffer is released, because the engine reuses it.
func (e *wasapiEngine) drain(sink sink) error {
	for {
		var packets uint32
		if err := comCall(e.capture, methodGetNextPacketSize, "IAudioCaptureClient.GetNextPacketSize",
			uintptr(unsafe.Pointer(&packets))); err != nil {
			return err
		}
		if packets == 0 {
			return nil
		}

		var buffer *byte
		var frames uint32
		var flags uint32
		if err := comCall(e.capture, methodGetBuffer, "IAudioCaptureClient.GetBuffer",
			uintptr(unsafe.Pointer(&buffer)), uintptr(unsafe.Pointer(&frames)),
			uintptr(unsafe.Pointer(&flags)), 0, 0); err != nil {
			return err
		}
		chunk := e.convert(buffer, frames, flags)
		if err := comCall(e.capture, methodReleaseBuffer, "IAudioCaptureClient.ReleaseBuffer",
			uintptr(frames)); err != nil {
			return err
		}
		if len(chunk) > 0 {
			sink.push(chunk)
		}
	}
}

// convert turns one packet of device frames into 16 kHz mono Int16.
func (e *wasapiEngine) convert(buffer *byte, frames uint32, flags uint32) []int16 {
	if frames == 0 || buffer == nil {
		return nil
	}
	samples := int(frames) * e.channels

	if flags&audclntBufferflagsSilent != 0 {
		// The engine has nothing to give. Real zeros go through the resampler
		// rather than skipping frames, so the sample clock keeps up with the
		// room and a muted minute does not shorten the capture.
		return e.resampler.float32Frames(make([]float32, samples), e.channels)
	}
	if e.kind == kindInt16 {
		return e.resampler.int16Frames(unsafe.Slice((*int16)(unsafe.Pointer(buffer)), samples), e.channels)
	}
	return e.resampler.float32Frames(unsafe.Slice((*float32)(unsafe.Pointer(buffer)), samples), e.channels)
}

// readMixFormat copies the engine's mix format out of COM and frees COM's
// copy: Initialize wants the original layout, extension and all, so the bytes
// are kept verbatim in the engine rather than reassembled.
func readMixFormat(client unsafe.Pointer) ([]byte, error) {
	var ptr *waveFormatEx
	if err := comCall(client, methodGetMixFormat, "IAudioClient.GetMixFormat",
		uintptr(unsafe.Pointer(&ptr))); err != nil {
		return nil, err
	}
	defer procCoTaskMemFree.Call(uintptr(unsafe.Pointer(ptr)))
	if ptr == nil {
		return nil, fmt.Errorf("IAudioClient.GetMixFormat returned no format")
	}

	size := 18 + int(ptr.Size)
	if size < 18 || size > 512 {
		return nil, fmt.Errorf("IAudioClient.GetMixFormat returned a %d byte format", size)
	}
	raw := make([]byte, size)
	copy(raw, unsafe.Slice((*byte)(unsafe.Pointer(ptr)), size))
	return raw, nil
}

// mixInfo is the part of a mix format the capture loop cares about.
type mixInfo struct {
	rate     int
	channels int
	kind     sampleKind
}

// parseMix reads the two codecs the engine can hand over: 32-bit float, which
// is what shared mode actually runs at, and 16-bit PCM. Anything else (24-bit
// packed in 32, for one) is refused here rather than decoded wrongly later.
func parseMix(raw []byte) (mixInfo, error) {
	if len(raw) < 18 {
		return mixInfo{}, fmt.Errorf("mix format is only %d bytes", len(raw))
	}
	tag := binary.LittleEndian.Uint16(raw[0:2])
	channels := int(binary.LittleEndian.Uint16(raw[2:4]))
	rate := int(binary.LittleEndian.Uint32(raw[4:8]))
	bits := int(binary.LittleEndian.Uint16(raw[14:16]))

	if channels < 1 || channels > 64 {
		return mixInfo{}, fmt.Errorf("mix format reports %d channels", channels)
	}
	if rate <= 0 {
		return mixInfo{}, fmt.Errorf("mix format reports %d Hz", rate)
	}

	kind := sampleKind(-1)
	switch tag {
	case waveFormatIeeeFloat:
		if bits == 32 {
			kind = kindFloat32
		}
	case waveFormatPcm:
		if bits == 16 {
			kind = kindInt16
		}
	case waveFormatExtensible:
		// WAVEFORMATEXTENSIBLE carries the real codec as the first word of
		// its sub-format GUID: 3 for float, 1 for PCM.
		if len(raw) >= 40 {
			switch binary.LittleEndian.Uint32(raw[24:28]) {
			case 3:
				if bits == 32 {
					kind = kindFloat32
				}
			case 1:
				if bits == 16 {
					kind = kindInt16
				}
			}
		}
	}
	if kind < 0 {
		return mixInfo{}, fmt.Errorf("mix format tag 0x%04X at %d bits is not supported", tag, bits)
	}
	return mixInfo{rate: rate, channels: channels, kind: kind}, nil
}
