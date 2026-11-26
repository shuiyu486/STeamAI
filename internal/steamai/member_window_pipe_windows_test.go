//go:build windows

package steamai

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

const memberWindowPipeHelperEnv = "STEAMAI_TEST_MEMBER_WINDOW_PIPE_HELPER"

var (
	procMemberWindowTestHandleCount = kernel32.NewProc("GetProcessHandleCount")
	memberWindowFixture             = memberWindowRequest{CaseID: "00000000:0000000000000001", Name: "static-analysis", Tier: tierStandard}
)

func memberWindowFrame(body string) []byte {
	frame := []byte{byte(len(body)), byte(len(body) >> 8)}
	return append(frame, body...)
}

func processHandleCount(t *testing.T) int {
	t.Helper()
	self, err := syscall.GetCurrentProcess()
	if err != nil {
		t.Fatal(err)
	}
	var count uint32
	if ok, _, callErr := procMemberWindowTestHandleCount.Call(uintptr(self), uintptr(unsafe.Pointer(&count))); ok == 0 {
		t.Fatalf("GetProcessHandleCount: %v", callErr)
	}
	return int(count)
}

// rawMemberWindowExchange 用正式客户端的拨号直接发一帧，原样返回回执。
func rawMemberWindowExchange(t *testing.T, address string, frame []byte) string {
	t.Helper()
	name, err := syscall.UTF16PtrFromString(address)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := dialMemberWindowPipe(name, time.Now().Add(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write(frame); err != nil {
		t.Fatal(err)
	}
	reply, err := io.ReadAll(io.LimitReader(conn, memberWindowMaxReply+1))
	if err != nil || len(reply) == 0 {
		t.Fatalf("没有收到回执: %v", err)
	}
	return string(reply)
}

func TestMemberWindowPipeDeliversOneRequestPerConnection(t *testing.T) {
	received := make(chan memberWindowRequest, 2)
	address, closeServer, err := serveMemberWindows(func(request memberWindowRequest) error {
		received <- request
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer closeServer()
	if !memberWindowAddressPattern.MatchString(address) || !strings.HasPrefix(address, memberWindowPipePrefix) {
		t.Fatalf("入口地址 %q 不是本机固定前缀下的随机管道名", address)
	}
	for i := 0; i < 2; i++ {
		if err := requestMemberWindow(address, memberWindowFixture); err != nil {
			t.Fatalf("第 %d 次请求: %v", i+1, err)
		}
		if got := <-received; got != memberWindowFixture {
			t.Fatalf("启动器收到 %+v，应为 %+v", got, memberWindowFixture)
		}
	}
}

func TestMemberWindowPipeStopsCleanlyAndReportsMissingLauncher(t *testing.T) {
	var calls atomic.Int32
	address, closeServer, err := serveMemberWindows(func(memberWindowRequest) error {
		calls.Add(1)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := requestMemberWindow(address, memberWindowFixture); err != nil {
		t.Fatal(err)
	}
	closeServer()
	closeServer() // 幂等。
	if err := requestMemberWindow(address, memberWindowFixture); err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("入口关闭后应明确报告启动器已不存在，得到 %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("入口关闭后仍处理了请求: 共 %d 次", calls.Load())
	}
}

func TestMemberWindowPipeRejectsRequestsOutsideTheContract(t *testing.T) {
	var calls atomic.Int32
	address, closeServer, err := serveMemberWindows(func(memberWindowRequest) error {
		calls.Add(1)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer closeServer()
	const head = `{"caseId":"c","name":"n","tier":"standard"`
	for _, c := range []struct{ label, body, contains string }{
		{"configDir", head + `,"configDir":"C:\\other"}`, "configDir"},
		{"命令", head + `,"exe":"placeholder.exe","args":["--flag"]}`, "unknown field"},
		{"环境", head + `,"env":["X=1"]}`, "unknown field"},
		{"不是对象", `["static-analysis"]`, ""},
		{"多个对象", head + `}{}`, ""},
		{"JSON null", `null`, "缺少"},
		{"字段为空", `{"caseId":"c","name":"","tier":"standard"}`, "缺少"},
		{"空请求", "", "长度"},
	} {
		reply := rawMemberWindowExchange(t, address, memberWindowFrame(c.body))
		if reply[0] != memberWindowReplyFail {
			t.Errorf("%s: 应拒绝，回执 %q", c.label, reply)
		}
		if c.contains != "" && !strings.Contains(reply, c.contains) {
			t.Errorf("%s: 回执应说明 %q，得到 %q", c.label, c.contains, reply)
		}
	}
	// 超过长度上限的帧（2049 = 0x0801）在读正文之前即拒绝，客户端不必再发正文。
	if reply := rawMemberWindowExchange(t, address, []byte{0x01, 0x08}); !strings.Contains(reply, "长度") {
		t.Errorf("超长帧应报长度无效，得到 %q", reply)
	}
	if calls.Load() != 0 {
		t.Fatalf("合同之外的请求不得走到窗口操作: 共 %d 次", calls.Load())
	}
}

func TestMemberWindowPipeReportsOpenFailureOnce(t *testing.T) {
	var calls atomic.Int32
	failure := errors.New("成员窗口请求与当前项目或档位不符")
	address, closeServer, err := serveMemberWindows(func(memberWindowRequest) error {
		calls.Add(1)
		return failure
	})
	if err != nil {
		t.Fatal(err)
	}
	defer closeServer()
	if err := requestMemberWindow(address, memberWindowFixture); err == nil || err.Error() != failure.Error() {
		t.Fatalf("应原样回传窗口操作的失败，得到 %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("窗口操作被调用 %d 次，失败的请求不得重放", calls.Load())
	}
}

func TestMemberWindowPipeDropsClientsThatLeaveWithoutARequest(t *testing.T) {
	previous := memberWindowSendWait
	memberWindowSendWait = 200 * time.Millisecond
	t.Cleanup(func() { memberWindowSendWait = previous })
	var calls atomic.Int32
	busy, entered := make(chan struct{}), make(chan struct{}, 1)
	address, closeServer, err := serveMemberWindows(func(memberWindowRequest) error {
		entered <- struct{}{}
		<-busy
		calls.Add(1)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer closeServer()
	name, err := syscall.UTF16PtrFromString(address)
	if err != nil {
		t.Fatal(err)
	}
	// 启动器正忙时，排队用的监听实例被连上后立刻离开：入口必须换实例继续，不能随之消失。
	served := make(chan error, 1)
	go func() { served <- requestMemberWindow(address, memberWindowFixture) }()
	<-entered
	leaver, err := dialMemberWindowPipe(name, time.Now().Add(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	leaver.Close()
	close(busy)
	if err := <-served; err != nil {
		t.Fatalf("先到的请求: %v", err)
	}
	// 连上却不发请求：到时限后服务端关闭连接，不调用窗口操作。
	idle, err := dialMemberWindowPipe(name, time.Now().Add(10*time.Second))
	if err != nil {
		t.Fatalf("客户端离开后入口不可用: %v", err)
	}
	defer idle.Close()
	_ = idle.SetReadDeadline(time.Now().Add(10 * time.Second))
	if _, err := io.ReadAll(idle); err != nil {
		t.Fatalf("服务端没有按时限断开不发请求的客户端: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("不发请求的客户端触发了窗口操作: 共 %d 次", calls.Load())
	}
	// 入口随后仍可用。
	if err := requestMemberWindow(address, memberWindowFixture); err != nil {
		t.Fatalf("断开空闲客户端后入口不可用: %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("窗口操作共 %d 次，应为 2 次", calls.Load())
	}
}

// memberWindowConcurrentRound 开一个入口，发 4 个并发请求后关闭；任何请求失败都算错误。
func memberWindowConcurrentRound(t *testing.T, open func(memberWindowRequest) error) {
	t.Helper()
	address, closeServer, err := serveMemberWindows(open)
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 4)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results <- requestMemberWindow(address, memberWindowRequest{CaseID: "c", Name: fmt.Sprintf("member-%d", i), Tier: tierHigh})
		}(i)
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Errorf("并发请求: %v", err)
		}
	}
	closeServer()
}

func TestMemberWindowPipeSerializesConcurrentRequestsAndReleasesEverything(t *testing.T) {
	var calls, active, overlap atomic.Int32
	open := func(memberWindowRequest) error {
		if active.Add(1) != 1 {
			overlap.Add(1)
		}
		time.Sleep(20 * time.Millisecond)
		active.Add(-1)
		calls.Add(1)
		return nil
	}
	// 运行时会为新线程惰性建句柄：并发预热到句柄数不再变化，再取基线。
	stable := processHandleCount(t)
	for round := 0; round < 6; round++ {
		memberWindowConcurrentRound(t, open)
		if current := processHandleCount(t); current == stable {
			break
		} else {
			stable = current
		}
	}
	before := calls.Load()
	baselineHandles, baselineGoroutines := processHandleCount(t), runtime.NumGoroutine()
	const cycles = 3
	for cycle := 0; cycle < cycles; cycle++ {
		memberWindowConcurrentRound(t, open)
	}
	if got := calls.Load() - before; got != cycles*4 {
		t.Errorf("窗口操作共 %d 次，应为 %d 次", got, cycles*4)
	}
	if overlap.Load() != 0 {
		t.Errorf("窗口操作被并发调用 %d 次", overlap.Load())
	}
	if goroutines := runtime.NumGoroutine(); goroutines > baselineGoroutines {
		t.Errorf("goroutine 从 %d 变为 %d", baselineGoroutines, goroutines)
	}
	// 留出运行时惰性句柄的少量余量；每轮入口若泄漏管道或事件，至少多出 3 个句柄。
	if handles := processHandleCount(t); handles > baselineHandles+3 {
		t.Errorf("进程句柄从 %d 变为 %d", baselineHandles, handles)
	}
}

func TestMemberWindowPipeRefusesForeignAddressAndSquatting(t *testing.T) {
	random := strings.Repeat("a", 40)
	for _, address := range []string{
		`\\host\pipe\steamai-member-window-` + random, memberWindowPipePrefix + "short",
		memberWindowPipePrefix + strings.Repeat("A", 40), memberWindowPipePrefix + strings.ToUpper(random),
		memberWindowWeztermMux, `\\.\pipe\other-` + random,
	} {
		if err := requestMemberWindow(address, memberWindowFixture); err == nil || !strings.Contains(err.Error(), "地址无效") {
			t.Errorf("地址 %q 应被拒绝，得到 %v", address, err)
		}
	}
	// 冒领：同名管道已存在时，不能把它当成自己建的首实例。
	name, err := syscall.UTF16PtrFromString(memberWindowPipePrefix + strings.Repeat("b", 40))
	if err != nil {
		t.Fatal(err)
	}
	squat := &memberWindowServer{name: name}
	pipe, err := squat.createPipe(true)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(pipe)
	if _, err := squat.createPipe(true); !errors.Is(err, syscall.ERROR_ACCESS_DENIED) {
		t.Fatalf("同名首实例应被拒绝，得到 %v", err)
	}
}

// memberWindowChildLines 把启动器子进程的输出逐行送入通道，读不动即结束。
func memberWindowChildLines(output io.Reader) <-chan string {
	lines := make(chan string, 1)
	go func() {
		defer close(lines)
		reader := bufio.NewReader(output)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			lines <- strings.TrimSpace(line)
		}
	}()
	return lines
}

// awaitMemberWindowLine 在限时内等到一行满足条件的输出；子进程提前结束或超时都算失败。
func awaitMemberWindowLine(lines <-chan string, accept func(string) bool) error {
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				return errors.New("启动器子进程提前结束")
			}
			if accept(line) {
				return nil
			}
		case <-timer.C:
			return errors.New("等待启动器输出超时")
		}
	}
}

// 启动器在处理请求途中死亡：客户端必须得到明确的“没有回执”，既不挂住也不重发。
func TestMemberWindowPipeLauncherDeathIsReported(t *testing.T) {
	if os.Getenv(memberWindowPipeHelperEnv) == "1" {
		// 子进程受理请求后回一行固定信号，然后停住不回执，直到被父测试结束。
		address, _, err := serveMemberWindows(func(memberWindowRequest) error {
			fmt.Println("opened")
			time.Sleep(time.Hour)
			return nil
		})
		if err != nil {
			os.Exit(3)
		}
		fmt.Println(address)
		time.Sleep(time.Hour)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(exe, "-test.run=^TestMemberWindowPipeLauncherDeathIsReported$")
	child.Env = append(os.Environ(), memberWindowPipeHelperEnv+"=1")
	output, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = child.Process.Kill()
		_ = child.Wait()
	}()
	lines := memberWindowChildLines(output)
	var address string
	if err := awaitMemberWindowLine(lines, func(line string) bool {
		if memberWindowAddressPattern.MatchString(line) {
			address = line
			return true
		}
		return false
	}); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { result <- requestMemberWindow(address, memberWindowFixture) }()
	// 等到启动器报告“请求已进入窗口操作”再结束它，确认的是处理途中死亡这一刻。
	if err := awaitMemberWindowLine(lines, func(line string) bool { return line == "opened" }); err != nil {
		t.Fatal(err)
	}
	if err := child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err == nil || !strings.Contains(err.Error(), "不会重发") {
			t.Fatalf("启动器死亡后应明确报告没有回执，得到 %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("启动器死亡后客户端仍然挂起")
	}
}
