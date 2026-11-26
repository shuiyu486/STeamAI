//go:build windows

package steamai

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"
	"unsafe"
)

// 成员窗口通道：会话运行在严格的 Job 里，工具无权在 Job 外创建窗口，所以会话内的
// steamai __open-member 只能把“打开本项目这名成员”交给创建它的 STeamAI 启动器（Job 外）。
// 通道是启动器为本次会话新建的一条本机命名管道：一条连接只承载一次请求和一次回执，
// 请求里只有项目标识、成员名和档位；命令、目录、配置域都由启动器自己决定，不从请求读取。

const (
	memberWindowPipePrefix = `\\.\pipe\steamai-member-window-`
	// 请求 = 2 字节小端长度 + JSON，回执 = 1 字节状态 + 文本；超过上限即拒绝。
	memberWindowMaxRequest = 2048
	memberWindowMaxReply   = 1024
	memberWindowReplyOK    = '+'
	memberWindowReplyFail  = '-'

	pipeAccessDuplex = 0x00000003
	// 以下两项属于 CreateNamedPipe 的管道模式（不是打开模式）：字节流等待，并拒绝远程客户端。
	pipeWait                = 0x00000000
	pipeRejectRemoteClients = 0x00000008
	fileFlagFirstInstance   = 0x00080000
	// 客户端只表明身份，不授予启动器递归模拟自己的权限。
	securitySQOSPresent    = 0x00100000
	securityIdentification = 0x00010000

	errorPipeBusy      syscall.Errno = 231
	errorPipeConnected syscall.Errno = 535
)

var (
	// 客户端连上后必须在此时间内发完请求，否则不调用窗口操作，直接断开。
	memberWindowSendWait = 5 * time.Second
	// 客户端从连接到收到回执的总时限，须覆盖排队和启动器的一次窗口操作（WezTerm CLI 自身 15 秒）。
	memberWindowRequestWait = 30 * time.Second

	// 只承认本机固定前缀加启动器生成的随机名；远程主机、其它前缀或被改写的值都不连接。
	memberWindowAddressPattern = regexp.MustCompile(`^\\\\\.\\pipe\\steamai-member-window-[0-9a-f]{40}$`)

	procMemberWindowCreateNamedPipe  = kernel32.NewProc("CreateNamedPipeW")
	procMemberWindowConnectNamedPipe = kernel32.NewProc("ConnectNamedPipe")
	procMemberWindowWaitNamedPipe    = kernel32.NewProc("WaitNamedPipeW")
	procMemberWindowCreateEvent      = kernel32.NewProc("CreateEventW")
	procMemberWindowSetEvent         = kernel32.NewProc("SetEvent")
	procMemberWindowResetEvent       = kernel32.NewProc("ResetEvent")
	procMemberWindowWaitForMultiple  = kernel32.NewProc("WaitForMultipleObjects")
	procMemberWindowSecurityFromSDDL = advapi32.NewProc("ConvertStringSecurityDescriptorToSecurityDescriptorW")
)

// memberWindowServer 是启动器一侧的通道。stop 通知 goroutine 退出，event 接收
// ConnectNamedPipe 的完成通知；两个事件和管道句柄都不交给会话进程。
type memberWindowServer struct {
	name  *uint16
	stop  syscall.Handle
	event syscall.Handle
	sa    *syscall.SecurityAttributes
	done  chan struct{}
	once  sync.Once
}

// serveMemberWindows 建立本会话的成员窗口入口：管道名每次随机，首实例同步建好才返回，
// 失败即在会话启动前报错。open 只由唯一的服务 goroutine 串行调用。closeServer 幂等，
// 等 goroutine 退出后才返回，因此返回时间以正在进行的 open 为界。
func serveMemberWindows(open func(memberWindowRequest) error) (string, func(), error) {
	var random [20]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", nil, fmt.Errorf("生成成员窗口管道名: %w", err)
	}
	address := memberWindowPipePrefix + hex.EncodeToString(random[:])
	name, err := syscall.UTF16PtrFromString(address)
	if err != nil {
		return "", nil, err
	}
	s := &memberWindowServer{name: name, done: make(chan struct{})}
	stop, _, callErr := procMemberWindowCreateEvent.Call(0, 1, 0, 0)
	if stop == 0 {
		return "", nil, fmt.Errorf("创建成员窗口通道停止事件: %w", callErr)
	}
	s.stop = syscall.Handle(stop)
	event, _, callErr := procMemberWindowCreateEvent.Call(0, 1, 0, 0)
	if event == 0 {
		s.release()
		return "", nil, fmt.Errorf("创建成员窗口通道完成事件: %w", callErr)
	}
	s.event = syscall.Handle(event)
	if s.sa, err = memberWindowPipeSecurity(); err != nil {
		s.release()
		return "", nil, err
	}
	pipe, err := s.createPipe(true)
	if err != nil {
		s.release()
		return "", nil, fmt.Errorf("创建成员窗口管道: %w", err)
	}
	go s.serve(pipe, open)
	return address, s.stopAndWait, nil
}

// stopAndWait 由 closeServer 独占事件和访问控制描述符的释放：先置停止事件、等服务
// goroutine 退出，再释放。goroutine 关闭 done 之后不再碰任何句柄，不存在关闭后的复用。
func (s *memberWindowServer) stopAndWait() {
	s.once.Do(func() {
		_, _, _ = procMemberWindowSetEvent.Call(uintptr(s.stop))
		<-s.done
		s.release()
	})
}

// stopping 查看停止事件是否已置位；手动复位事件不会被这次查看清除。
func (s *memberWindowServer) stopping() bool {
	state, err := syscall.WaitForSingleObject(s.stop, 0)
	return err == nil && state == syscall.WAIT_OBJECT_0
}

// release 释放服务期一直占用的访问控制描述符和事件。
func (s *memberWindowServer) release() {
	if s.sa != nil {
		_, _ = syscall.LocalFree(syscall.Handle(s.sa.SecurityDescriptor))
		s.sa = nil
	}
	for _, handle := range []syscall.Handle{s.stop, s.event} {
		if handle != 0 {
			_ = syscall.CloseHandle(handle)
		}
	}
	s.stop, s.event = 0, 0
}

// memberWindowPipeSecurity 把管道限定为当前用户和 SYSTEM。主体取自当前进程 token 的用户 SID，
// 不依赖对象 owner：管理员运行时新建对象的 owner 可能是组。
func memberWindowPipeSecurity() (*syscall.SecurityAttributes, error) {
	token, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		return nil, fmt.Errorf("读取当前进程身份: %w", err)
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return nil, fmt.Errorf("读取当前用户: %w", err)
	}
	sid, err := user.User.Sid.String()
	if err != nil {
		return nil, fmt.Errorf("读取当前用户 SID: %w", err)
	}
	sddl, err := syscall.UTF16PtrFromString("D:P(A;;GA;;;SY)(A;;GA;;;" + sid + ")")
	if err != nil {
		return nil, err
	}
	var descriptor unsafe.Pointer
	if ok, _, callErr := procMemberWindowSecurityFromSDDL.Call(uintptr(unsafe.Pointer(sddl)), 1, uintptr(unsafe.Pointer(&descriptor)), 0); ok == 0 || descriptor == nil {
		return nil, fmt.Errorf("成员窗口管道访问控制: %w", callErr)
	}
	return &syscall.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(syscall.SecurityAttributes{})),
		SecurityDescriptor: uintptr(descriptor),
	}, nil
}

// createPipe 新建一个监听实例。首实例带 FIRST_INSTANCE：管道名已被别人占用即失败，
// 不会接到冒领者后面；所有实例都拒绝远程连接。最多两个实例：正在处理的一个加一个监听。
func (s *memberWindowServer) createPipe(first bool) (syscall.Handle, error) {
	openMode := uint32(pipeAccessDuplex | syscall.FILE_FLAG_OVERLAPPED)
	if first {
		openMode |= fileFlagFirstInstance
	}
	handle, _, callErr := procMemberWindowCreateNamedPipe.Call(
		uintptr(unsafe.Pointer(s.name)), uintptr(openMode), uintptr(pipeWait|pipeRejectRemoteClients), 2,
		memberWindowMaxReply+2, memberWindowMaxRequest+2, 0, uintptr(unsafe.Pointer(s.sa)))
	if syscall.Handle(handle) == syscall.InvalidHandle {
		return 0, callErr
	}
	return syscall.Handle(handle), nil
}

func (s *memberWindowServer) serve(pipe syscall.Handle, open func(memberWindowRequest) error) {
	defer close(s.done)
	for {
		connected := s.awaitConnection(pipe)
		if !connected && s.stopping() {
			_ = syscall.CloseHandle(pipe)
			return
		}
		// 先建好下一个监听实例再处理或丢弃当前实例：启动器存活期间管道名始终存在，
		// 后来的请求在内核里排队，不会被误判成启动器已退出。
		next, err := s.createPipe(false)
		if connected {
			s.serveConnection(pipe, open)
		} else {
			// 客户端在开始监听前连上又离开，这个实例已失效；换一个新的继续，不让入口消失。
			_ = syscall.CloseHandle(pipe)
			time.Sleep(10 * time.Millisecond) // 监听反复失败时不空转。
		}
		if err != nil {
			return
		}
		pipe = next
	}
}

// awaitConnection 等一个客户端连上；已请求停止或管道失效时返回 false。
func (s *memberWindowServer) awaitConnection(pipe syscall.Handle) bool {
	_, _, _ = procMemberWindowResetEvent.Call(uintptr(s.event))
	overlapped := &syscall.Overlapped{HEvent: s.event}
	// 挂起期间内核持有 OVERLAPPED：固定到 I/O 真正结束，不把可移动的地址借给内核。
	var pinned runtime.Pinner
	pinned.Pin(overlapped)
	defer pinned.Unpin()
	connected, _, callErr := procMemberWindowConnectNamedPipe.Call(uintptr(pipe), uintptr(unsafe.Pointer(overlapped)))
	errno, _ := callErr.(syscall.Errno)
	if errno != syscall.ERROR_IO_PENDING {
		if connected == 0 && errno != errorPipeConnected {
			return false // 管道已失效。
		}
		return !s.stopping() // 已连上的快路同样服从停止请求。
	}
	handles := [2]syscall.Handle{s.stop, s.event}
	result, _, _ := procMemberWindowWaitForMultiple.Call(2, uintptr(unsafe.Pointer(&handles[0])), 0, uintptr(syscall.INFINITE))
	if result == syscall.WAIT_OBJECT_0+1 {
		return true
	}
	// 取消挂起的连接请求并等它真正结束：I/O 结束前不能动 OVERLAPPED，也不能关句柄。
	_ = syscall.CancelIoEx(pipe, overlapped)
	_, _ = syscall.WaitForSingleObject(s.event, syscall.INFINITE)
	return false
}

// serveConnection 处理唯一一条请求并回执，随后关闭该实例。句柄以 overlapped 打开，
// 交给 os.File 后可按时限等待，不发请求或不读回执的客户端都卡不住服务。
func (s *memberWindowServer) serveConnection(pipe syscall.Handle, open func(memberWindowRequest) error) {
	conn := os.NewFile(uintptr(pipe), memberWindowPipePrefix)
	defer conn.Close()
	if conn.SetReadDeadline(time.Now().Add(memberWindowSendWait)) != nil {
		return
	}
	request, err := decodeMemberWindowRequest(conn)
	if err == nil {
		err = open(request)
	}
	_ = conn.SetWriteDeadline(time.Now().Add(memberWindowSendWait))
	_, _ = conn.Write(memberWindowReply(err))
}

// memberWindowReply 把窗口操作的结果编成一条有界回执：状态在前，文本原样给用户看。
func memberWindowReply(err error) []byte {
	reply := []byte{memberWindowReplyOK}
	if err == nil {
		return reply
	}
	reply[0] = memberWindowReplyFail
	reply = append(reply, err.Error()...)
	if len(reply) > memberWindowMaxReply {
		reply = reply[:memberWindowMaxReply]
		for !utf8.Valid(reply) { // 截断处不留半个字符。
			reply = reply[:len(reply)-1]
		}
	}
	return reply
}

// decodeMemberWindowRequest 读取唯一一条请求：长度有界，必须是单个对象，只接受
// caseId/name/tier 三个字段且都不为空。多出的 configDir、命令、环境等字段在读入阶段即拒绝，
// 不会走到窗口操作。
func decodeMemberWindowRequest(conn io.Reader) (memberWindowRequest, error) {
	var frame [2]byte
	if _, err := io.ReadFull(conn, frame[:]); err != nil {
		return memberWindowRequest{}, errors.New("成员窗口入口没有收到完整请求")
	}
	size := int(binary.LittleEndian.Uint16(frame[:]))
	if size == 0 || size > memberWindowMaxRequest {
		return memberWindowRequest{}, errors.New("成员窗口请求长度无效")
	}
	body := make([]byte, size)
	if _, err := io.ReadFull(conn, body); err != nil {
		return memberWindowRequest{}, errors.New("成员窗口入口没有收到完整请求")
	}
	var request memberWindowRequest
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	// JSON null 或缺字段都能解码成功：三项目标识缺一不可，不让空请求走到窗口操作。
	if err := decoder.Decode(&request); err != nil {
		return memberWindowRequest{}, fmt.Errorf("成员窗口请求无效: %v", err)
	}
	if err := decoder.Decode(new(json.RawMessage)); err != io.EOF {
		return memberWindowRequest{}, errors.New("成员窗口请求无效: 必须是单个 JSON 对象")
	}
	if request.CaseID == "" || request.Name == "" || request.Tier == "" {
		return memberWindowRequest{}, errors.New("成员窗口请求无效: 缺少项目、成员名或档位")
	}
	return request, nil
}

// requestMemberWindow 把一次成员窗口请求交给启动器并等待回执。请求最多发出一次：
// 发出之后的超时或断开都如实报告，不重发，因为启动器可能已经打开了窗口。
func requestMemberWindow(address string, request memberWindowRequest) error {
	if !memberWindowAddressPattern.MatchString(address) {
		return errors.New("成员窗口通道地址无效：只接受本机 STeamAI 启动器创建的管道")
	}
	body, err := json.Marshal(request)
	if err != nil || len(body) > memberWindowMaxRequest {
		return errors.New("成员窗口请求无法编码")
	}
	name, err := syscall.UTF16PtrFromString(address)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(memberWindowRequestWait)
	conn, err := dialMemberWindowPipe(name, deadline)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("设置成员窗口请求时限: %w", err)
	}
	frame := []byte{byte(len(body)), byte(len(body) >> 8)}
	if _, err := conn.Write(append(frame, body...)); err != nil {
		return fmt.Errorf("成员窗口请求没有发出: %w", err)
	}
	reply, err := io.ReadAll(io.LimitReader(conn, memberWindowMaxReply+1))
	switch {
	case errors.Is(err, os.ErrDeadlineExceeded):
		return fmt.Errorf("等待成员窗口入口回执超过 %s；请求已发出且不会重发，窗口可能仍会被打开", memberWindowRequestWait)
	case err != nil:
		return fmt.Errorf("成员窗口入口在回执前断开（%w）；请求已发出且不会重发，窗口是否已打开未知", err)
	case len(reply) == 0 || len(reply) > memberWindowMaxReply:
		return errors.New("成员窗口入口在回执前关闭；请求已发出且不会重发，窗口是否已打开未知")
	case reply[0] != memberWindowReplyOK:
		return errors.New(string(reply[1:]))
	}
	return nil
}

// dialMemberWindowPipe 连接启动器的管道。入口不存在说明启动器已退出；正忙时在时限内
// 等待并重连，此时请求还没有发出，不算重放。
func dialMemberWindowPipe(name *uint16, deadline time.Time) (*os.File, error) {
	for {
		handle, err := syscall.CreateFile(name, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil,
			syscall.OPEN_EXISTING, syscall.FILE_FLAG_OVERLAPPED|securitySQOSPresent|securityIdentification, 0)
		if err == nil {
			return os.NewFile(uintptr(handle), memberWindowPipePrefix), nil
		}
		if errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) {
			return nil, errors.New("成员窗口入口已不存在：启动器可能已退出")
		}
		if !errors.Is(err, errorPipeBusy) {
			return nil, fmt.Errorf("连接成员窗口入口: %w", err)
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, errors.New("成员窗口入口正忙：本次请求没有发出")
		}
		if waited, _, _ := procMemberWindowWaitNamedPipe.Call(uintptr(unsafe.Pointer(name)), uintptr(remaining.Milliseconds())); waited == 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}
}
