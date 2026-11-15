#pragma once

#include <windows.h>

#include <deque>
#include <string>

namespace dm {

// UI 定时器驱动的 dm_worker 私有 stdio 管道。
// 只在按钮动作时写 stdin（pause/resume/stop）；读 stdout 用 PeekNamedPipe
// 先查再有界 ReadFile，绝不阻塞 UI 线程，不做通用进程运行器。
class WorkerProcess {
public:
    WorkerProcess() = default;
    ~WorkerProcess();
    bool start(const std::wstring& executable, const std::wstring& argumentLine, std::wstring& error,
               const std::wstring& desktop = {});
    // 每轮收走可用输出；输出通道关闭且进程结束时取退出码。只在 UI 线程调用。
    void poll();
    bool active() const { return process_ != nullptr; }
    bool running() const { return process_ != nullptr && !exited_; }
    bool exited() const { return exited_; }
    int exitCode() const { return exitCode_; }
    DWORD processId() const { return process_ ? GetProcessId(process_) : 0; }
    bool writeLine(const std::string& line);
    void takeLines(std::deque<std::wstring>& out);
    unsigned long ticks() const { return ticks_; }
    void reset();  // 退出后由 UI 调用，释放全部句柄并允许下一次启动
    WorkerProcess(const WorkerProcess&) = delete;
    WorkerProcess& operator=(const WorkerProcess&) = delete;

private:
    void closeAll();

    HANDLE process_ = nullptr;
    HANDLE stdinWrite_ = nullptr;   // 父私有写端；子进程不继承
    HANDLE stdoutRead_ = nullptr;   // 父私有读端；子进程不继承
    std::deque<std::wstring> lines_;
    std::string pending_;
    unsigned long ticks_ = 0;
    bool exited_ = false;
    int exitCode_ = 0;
    bool outputClosed_ = false;
};

} // namespace dm
