package bluetooth

import (
	"fmt"
	"sync"
	"syscall"
	"time"
	"unsafe"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

type _Socklen uint32
type RawSockaddrL2 struct {
	Family uint16
	Psm    uint16
	Bdaddr [6]uint8
}

// Represents L2CAP socket address
type SockaddrL2 struct {
	PSM    uint16
	Bdaddr [6]uint8
	raw    RawSockaddrL2
}

func (sa *SockaddrL2) sockaddr() (unsafe.Pointer, _Socklen, error) {
	sa.raw.Family = unix.AF_BLUETOOTH
	sa.raw.Psm = uint16(sa.PSM)
	sa.raw.Bdaddr = sa.Bdaddr

	return unsafe.Pointer(&sa.raw), _Socklen(unsafe.Sizeof(RawSockaddrL2{})), nil
}

func (sa *SockaddrL2) String() string {
	return fmt.Sprintf("[PSM: %d, Bdaddr: %v]", sa.PSM, sa.Bdaddr)
}

const (
	PSMCTRL = 0x11
	PSMINTR = 0x13
	BUFSIZE = 1024

	FDBITS = 32
)

var mu sync.Mutex

type FdSet struct {
	Bits [32]int32
}

func setFd(fd int, fdset *FdSet) {
	mask := uint(1) << (uint(fd) % uint(FDBITS))
	fdset.Bits[fd/FDBITS] |= int32(mask)
}

func isSetFd(fd int, fdset *FdSet) bool {
	mask := uint(1) << (uint(fd) % uint(FDBITS))
	return ((fdset.Bits[fd/FDBITS] & int32(mask)) != 0)
}

type Bluetooth struct {
	fd     int
	family int
	proto  int
	typ    int
	saddr  SockaddrL2

	block bool
	mu    sync.Mutex
}

// Sets socket as blocking mode(true) or Non-blocking mode(false)
func (bt *Bluetooth) SetBlocking(block bool) error {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	dFlgPtr, _, err := unix.Syscall(unix.SYS_FCNTL, uintptr(bt.fd), unix.F_GETFL, 0)

	if err != 0 {
		log.Debug("Error SetBlocking set state", err)
		return err
	}

	var delayFlag uint
	if block {
		delayFlag = uint(dFlgPtr) & (^(uint)(unix.O_NONBLOCK))
	} else {
		delayFlag = uint(dFlgPtr) | ((uint)(unix.O_NONBLOCK))
	}

	_, _, err = unix.Syscall(unix.SYS_FCNTL, uintptr(bt.fd), unix.F_SETFL, uintptr(delayFlag))
	if err != 0 {
		log.Debug("Error SetBlocking set state", err)
		return err
	}

	return nil
}

// Creates L2CAP socket wrapper with given file descriptor
// This file descriptor is provided by BlueZ DBus interface
// e.g. org.bluez.Profile1.NewConnection()
func NewBluetoothSocket(fd int) (*Bluetooth, error) {
	bt := &Bluetooth{
		fd:     fd,
		family: unix.AF_BLUETOOTH,
		typ:    unix.SOCK_SEQPACKET,
		proto:  unix.BTPROTO_L2CAP,
		block:  false,
	}

	var rsa RawSockaddrL2
	var addrlen _Socklen = _Socklen(unsafe.Sizeof(RawSockaddrL2{}))
	_, _, err := unix.RawSyscall(unix.SYS_GETSOCKNAME, uintptr(fd), uintptr(unsafe.Pointer(&rsa)), uintptr(unsafe.Pointer(&addrlen)))
	if int(err) != 0 {
		log.Debug("Failure on getsockname", err)
		unix.Close(fd)
		return nil, err
	}

	bt.saddr = SockaddrL2{
		PSM:    rsa.Psm,
		Bdaddr: rsa.Bdaddr,
	}

	log.Debug("Resolved sockname", bt.saddr, "New Socket is created")

	return bt, nil
}

// Creates L2CAP socket and lets it listen on given PSM
func Listen(psm uint, bklen int, block bool) (*Bluetooth, error) {
	mu.Lock()
	defer mu.Unlock()
	bt := &Bluetooth{
		family: unix.AF_BLUETOOTH,
		typ:    unix.SOCK_SEQPACKET, // RFCOMM = SOCK_STREAM, L2CAP = SOCK_SEQPACKET, HCI = SOCK_RAW
		proto:  unix.BTPROTO_L2CAP,
		block:  block,
	}

	fd, err := unix.Socket(bt.family, bt.typ, bt.proto)
	if err != nil {
		log.Debug("Socket could not be created", err)
		return nil, err
	}
	log.Debug("Socket is created")

	bt.fd = fd
	unix.CloseOnExec(bt.fd)

	if err := bt.SetBlocking(block); err != nil {
		_err := bt.Close()
		log.Debug("SetBlocking", _err, err)
		return nil, err
	}
	log.Debug("Socket is set blocking mode")

	// because L2CAP socket address struct does not exist in golang's standard libs
	// must be binded by using very low-level operations
	addr := SockaddrL2{
		PSM:    uint16(psm),
		Bdaddr: [6]uint8{0},
	}
	bt.saddr = addr
	saddr, saddrlen, err := addr.sockaddr()

	if _, _, err := unix.Syscall(unix.SYS_BIND, uintptr(bt.fd), uintptr(saddr), uintptr(saddrlen)); int(err) != 0 {
		switch int(err) {
		case 0:
		default:
			_err := bt.Close()
			log.Debug("Failure on Binding Socket", _err, err)
			return nil, err
		}
	}
	log.Debug("Socket is binded")

	if err := unix.Listen(bt.fd, bklen); err != nil {
		_err := bt.Close()
		log.Debug(_err)
		return nil, err
	}
	log.Debug("Socket is listening")

	return bt, nil
}

// Accepts on listening socket and return received connection
func (bt *Bluetooth) Accept() (*Bluetooth, error) {
	mu.Lock()
	defer mu.Unlock()

	var nFd int
	var rAddr *SockaddrL2

	fds := &FdSet{Bits: [32]int32{0}}
	setFd(bt.fd, fds)
	for {
		var raddr RawSockaddrL2
		var addrlen _Socklen = _Socklen(unsafe.Sizeof(RawSockaddrL2{}))
		rFd, _, err := unix.Syscall(unix.SYS_ACCEPT, uintptr(bt.fd), uintptr(unsafe.Pointer(&raddr)), uintptr(unsafe.Pointer(&addrlen)))
		if err != 0 {
			switch err {
			case syscall.EAGAIN:
				time.Sleep(1 * time.Millisecond)
				continue
			case syscall.ECONNABORTED:
				continue
			}
			log.Debug("Accept: Socket Error", err)
			unix.Close(int(rFd))
			return nil, err
		}

		nFd = int(rFd)
		rAddr = &SockaddrL2{
			PSM:    raddr.Psm,
			Bdaddr: raddr.Bdaddr,
		}
		break
	}

	log.Debug("Remote Address Info", rAddr)

	rbt := &Bluetooth{
		family: bt.family,
		typ:    bt.typ,
		proto:  bt.proto,
		block:  bt.block,
		fd:     nFd,
		saddr:  *rAddr,
	}

	unix.CloseOnExec(nFd)
	log.Debug("Accept closeonexec")

	if err := rbt.SetBlocking(false); err != nil {
		_err := bt.Close()
		_err = rbt.Close()
		log.Debug(_err, "SetBlocking", err)
		return nil, err
	}
	log.Debug("Accepted Socket could set blocking mode")

	return rbt, nil
}

func (bt *Bluetooth) Read(b []byte) (int, error) {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	var bp unsafe.Pointer
	var _zero uintptr
	if len(b) > 0 {
		bp = unsafe.Pointer(&b[0])
	} else {
		bp = unsafe.Pointer(&_zero)
	}
	fds := &FdSet{Bits: [32]int32{0}}
	setFd(bt.fd, fds)

	var r int
	for {
		_r, _, err := unix.Syscall(unix.SYS_READ, uintptr(bt.fd), uintptr(bp), uintptr(len(b)))

		if err != 0 {
			switch err {
			case syscall.EAGAIN:
				time.Sleep(1 * time.Millisecond)
				continue
			}
			log.Debug("Bluetooth Read Error", err)
			return -1, err
		}

		r = int(_r)
		break
	}

	return r, nil
}

func (bt *Bluetooth) Write(d []byte) (int, error) {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	var dp unsafe.Pointer
	var _zero uintptr
	if len(d) > 0 {
		dp = unsafe.Pointer(&d[0])
	} else {
		dp = unsafe.Pointer(&_zero)
	}
	fds := &FdSet{Bits: [32]int32{0}}
	setFd(bt.fd, fds)

	var r int
	for {
		_r, _, err := unix.Syscall(unix.SYS_WRITE, uintptr(bt.fd), uintptr(dp), uintptr(len(d)))

		if err != 0 {
			log.Debug("Bluetooth Write Error", err)
			return -1, err
		}

		r = int(_r)
		break
	}

	return r, nil
}

func (bt *Bluetooth) Close() error {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	if bt.fd <= 0 {
		return unix.EINVAL
	}

	if err := unix.Close(bt.fd); err != nil {
		log.Debug("Bluetooth Close fd Error", err)
		return err
	}

	return nil
}
