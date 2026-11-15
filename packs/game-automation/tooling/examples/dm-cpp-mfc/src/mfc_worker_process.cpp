#include "mfc_worker_process.h"

#include <vector>

namespace dm {

namespace {

std::wstring toWide(const std::string& text) {
    if (text.empty()) return {};
    const int size = MultiByteToWideChar(CP_UTF8, 0, text.data(), static_cast<int>(text.size()), nullptr, 0);
    std::wstring wide(size, L'\0');
    MultiByteToWideChar(CP_UTF8, 0, text.data(), static_cast<int>(text.size()), wide.data(), size);
    return wide;
}

} // namespace

WorkerProcess::~WorkerProcess() { closeAll(); }

void WorkerProcess::closeAll() {
    // 先关父私有 stdin 写端：worker 由此收到 EOF 并按既有合同自行停止。
    if (stdinWrite_) { CloseHandle(stdinWrite_); stdinWrite_ = nullptr; }
    if (stdoutRead_) { CloseHandle(stdoutRead_); stdoutRead_ = nullptr; }
    if (process_) { CloseHandle(process_); process_ = nullptr; }
}

bool WorkerProcess::start(const std::wstring& executable, const std::wstring& argumentLine,
                          std::wstring& error, const std::wstring& desktop) {
    if (process_) return false;
    reset();
    // 子进程只继承自己那三个 stdio 句柄；父侧两端保持私有（否则父端关闭后 worker 收不到 EOF）。
    SECURITY_ATTRIBUTES inheritable{sizeof(SECURITY_ATTRIBUTES), nullptr, TRUE};
    HANDLE childStdinRead = nullptr;
    HANDLE childStdoutWrite = nullptr;
    if (!CreatePipe(&childStdinRead, &stdinWrite_, &inheritable, 0) ||
        !CreatePipe(&stdoutRead_, &childStdoutWrite, &inheritable, 0)) {
        const DWORD lastError = GetLastError();
        if (childStdinRead) CloseHandle(childStdinRead);
        closeAll();
        error = L"CreatePipe failed: " + std::to_wstring(lastError);
        return false;
    }
    if (!SetHandleInformation(stdinWrite_, HANDLE_FLAG_INHERIT, 0) ||
        !SetHandleInformation(stdoutRead_, HANDLE_FLAG_INHERIT, 0)) {
        const DWORD lastError = GetLastError();
        CloseHandle(childStdinRead);
        CloseHandle(childStdoutWrite);
        closeAll();
        error = L"SetHandleInformation failed: " + std::to_wstring(lastError);
        return false;
    }
    std::wstring command = L"\"" + executable + L"\"";
    if (!argumentLine.empty()) command += L" " + argumentLine;
    std::vector<wchar_t> mutableCommand(command.begin(), command.end());
    mutableCommand.push_back(L'\0');
    STARTUPINFOEXW startup{};
    startup.StartupInfo.cb = sizeof(startup);
    startup.StartupInfo.dwFlags = STARTF_USESTDHANDLES;
    startup.StartupInfo.hStdInput = childStdinRead;
    startup.StartupInfo.hStdOutput = childStdoutWrite;
    startup.StartupInfo.hStdError = childStdoutWrite;
    startup.StartupInfo.lpDesktop = desktop.empty() ? nullptr : const_cast<wchar_t*>(desktop.c_str());
    // worker 再启动自建目标时，不让目标继承 MFC 的控制/日志管道。
    SIZE_T attributeBytes = 0;
    InitializeProcThreadAttributeList(nullptr, 1, 0, &attributeBytes);
    std::vector<unsigned char> attributes(attributeBytes);
    startup.lpAttributeList = reinterpret_cast<LPPROC_THREAD_ATTRIBUTE_LIST>(attributes.data());
    const bool initialized = InitializeProcThreadAttributeList(startup.lpAttributeList, 1, 0,
                                                               &attributeBytes) != FALSE;
    HANDLE childHandles[] = {childStdinRead, childStdoutWrite};
    if (!initialized || !UpdateProcThreadAttribute(startup.lpAttributeList, 0,
            PROC_THREAD_ATTRIBUTE_HANDLE_LIST, childHandles, sizeof(childHandles), nullptr, nullptr)) {
        const DWORD lastError = GetLastError();
        if (initialized) DeleteProcThreadAttributeList(startup.lpAttributeList);
        CloseHandle(childStdinRead);
        CloseHandle(childStdoutWrite);
        closeAll();
        error = L"Child handle list failed: " + std::to_wstring(lastError);
        return false;
    }
    const size_t slash = executable.find_last_of(L"\\/");
    const std::wstring directory = slash == std::wstring::npos ? std::wstring() : executable.substr(0, slash);
    PROCESS_INFORMATION processInfo{};
    const BOOL created = CreateProcessW(executable.c_str(), mutableCommand.data(), nullptr, nullptr, TRUE,
        CREATE_NO_WINDOW | EXTENDED_STARTUPINFO_PRESENT, nullptr,
        directory.empty() ? nullptr : directory.c_str(), &startup.StartupInfo, &processInfo);
    const DWORD lastError = created ? ERROR_SUCCESS : GetLastError();
    DeleteProcThreadAttributeList(startup.lpAttributeList);
    if (!created) {
        CloseHandle(childStdinRead);
        CloseHandle(childStdoutWrite);
        closeAll();
        error = L"CreateProcessW failed: " + std::to_wstring(lastError);
        return false;
    }
    CloseHandle(childStdinRead);
    CloseHandle(childStdoutWrite);
    CloseHandle(processInfo.hThread);
    process_ = processInfo.hProcess;
    exited_ = false;
    exitCode_ = 0;
    outputClosed_ = false;
    pending_.clear();
    lines_.clear();
    ticks_ = 0;
    return true;
}

void WorkerProcess::poll() {
    if (!process_ || exited_) return;
    if (!outputClosed_) {
        size_t remaining = 65536;
        while (remaining != 0) {
            DWORD available = 0;
            if (!PeekNamedPipe(stdoutRead_, nullptr, 0, nullptr, &available, nullptr)) {
                outputClosed_ = true;  // worker 已关闭 stdout
                break;
            }
            if (available == 0) break;
            char buffer[4096];
            const DWORD chunk = available < sizeof(buffer) ? available : static_cast<DWORD>(sizeof(buffer));
            const DWORD want = chunk < remaining ? chunk : static_cast<DWORD>(remaining);
            DWORD read = 0;
            if (!ReadFile(stdoutRead_, buffer, want, &read, nullptr) || read == 0) {
                outputClosed_ = true;
                break;
            }
            remaining -= read;
            pending_.append(buffer, read);
            size_t newline = std::string::npos;
            while ((newline = pending_.find('\n')) != std::string::npos) {
                std::string line = pending_.substr(0, newline);
                pending_.erase(0, newline + 1);
                if (!line.empty() && line.back() == '\r') line.pop_back();
                if (line == "TICK") { ++ticks_; continue; }
                if (line.size() > 8192) line = "worker output line too long (truncated)";
                lines_.push_back(toWide(line));
            }
            if (pending_.size() > 8192) {
                pending_.clear();
                lines_.push_back(L"ERROR child output line too long");
            }
            if (read < want) break;
        }
    }
    if (outputClosed_ && WaitForSingleObject(process_, 0) == WAIT_OBJECT_0) {
        DWORD code = 0;
        GetExitCodeProcess(process_, &code);
        exitCode_ = static_cast<int>(code);
        exited_ = true;
    }
}

bool WorkerProcess::writeLine(const std::string& line) {
    if (!stdinWrite_ || !process_) return false;
    std::string data = line + "\n";
    DWORD written = 0;
    const bool sent = WriteFile(stdinWrite_, data.data(), static_cast<DWORD>(data.size()), &written, nullptr) &&
                      written == data.size();
    SecureZeroMemory(data.data(), data.size());  // 初始化行可能含许可信息
    return sent;
}

void WorkerProcess::takeLines(std::deque<std::wstring>& out) {
    out.swap(lines_);
    lines_.clear();
}

void WorkerProcess::reset() {
    closeAll();
    exited_ = false;
    outputClosed_ = false;
    pending_.clear();
    lines_.clear();
}

} // namespace dm
