#include "window_demo.h"

#include "mfc_worker_process.h"

#include <chrono>
#include <deque>
#include <exception>
#include <sstream>
#include <stdexcept>

namespace dm {
namespace {

struct Stopped {};

class DemoTarget {
public:
    DemoTarget(const std::filesystem::path& executable, bool hidden) {
        if (!executable.is_absolute() || !std::filesystem::is_regular_file(executable))
            throw std::runtime_error("Demo target requires an existing absolute executable path");
        std::wstring error;
        if (!process_.start(executable.wstring(), hidden ? L"--hidden" : L"", error))
            throw std::runtime_error("Cannot start demo target");
    }

    HWND ready(Control& control) {
        const std::wstring line = receive(L"TARGET ready ", &control);
        std::wistringstream fields(line.substr(13));
        unsigned long long raw = 0;
        unsigned long pid = 0;
        std::wstring extra;
        if (!(fields >> raw >> pid) || (fields >> extra) || raw == 0 || raw > MAXDWORD ||
            pid != process_.processId())
            throw std::runtime_error("Invalid demo target identity");
        window_ = reinterpret_cast<HWND>(static_cast<ULONG_PTR>(raw));
        verify();
        return window_;
    }

    void verify() const {
        DWORD owner = 0;
        if (!window_ || !IsWindow(window_) || !GetWindowThreadProcessId(window_, &owner) ||
            owner != process_.processId() || !process_.running())
            throw std::runtime_error("Demo target window is no longer owned by our child");
    }

    struct Counts { long moves, clickDown, clickUp, keyDown, keyUp; };

    Counts stats(Control& control) {
        verify();
        if (!process_.writeLine("stats")) throw std::runtime_error("Cannot request target counts");
        const std::wstring line = receive(L"TARGET stats ", &control);
        std::wistringstream fields(line.substr(13));
        Counts counts{};
        std::wstring extra;
        if (!(fields >> counts.moves >> counts.clickDown >> counts.clickUp >> counts.keyDown >> counts.keyUp) ||
            (fields >> extra) || counts.moves < 0 || counts.clickDown < 0 || counts.clickUp < 0 ||
            counts.keyDown < 0 || counts.keyUp < 0)
            throw std::runtime_error("Invalid demo target counts");
        return counts;
    }

    void close() {
        if (!process_.active()) return;
        process_.poll();
        if (process_.running() && !process_.writeLine("close"))
            throw std::runtime_error("Cannot request target close");
        const auto deadline = std::chrono::steady_clock::now() + std::chrono::seconds(5);
        while (!process_.exited() && std::chrono::steady_clock::now() < deadline) {
            process_.poll();
            Sleep(10);
        }
        if (!process_.exited()) throw std::runtime_error("Demo target close timed out");
        if (process_.exitCode() != 0) throw std::runtime_error("Demo target exited with an error");
        if (window_ && IsWindow(window_)) {
            DWORD owner = 0;
            GetWindowThreadProcessId(window_, &owner);
            if (owner == process_.processId()) throw std::runtime_error("Demo target window still exists");
        }
        process_.reset();
    }

private:
    std::wstring receive(const wchar_t* prefix, Control* control) {
        const auto deadline = std::chrono::steady_clock::now() + std::chrono::seconds(5);
        while (std::chrono::steady_clock::now() < deadline) {
            if (control && !control->checkpoint()) throw Stopped{};
            process_.poll();
            std::deque<std::wstring> lines;
            process_.takeLines(lines);
            for (const auto& line : lines) {
                if (line.rfind(prefix, 0) == 0) return line;
                if (line.rfind(L"ERROR", 0) == 0) throw std::runtime_error("Demo target reported an error");
            }
            if (process_.exited()) throw std::runtime_error("Demo target exited before replying");
            Sleep(10);
        }
        throw std::runtime_error("Demo target reply timed out");
    }

    WorkerProcess process_;
    HWND window_ = nullptr;
};

void checkpoint(Control& control) {
    if (!control.checkpoint()) throw Stopped{};
}

} // namespace

void runWindowDemo(Session& session, Control& control, const std::filesystem::path& executable,
                   bool hidden, void (*report)(const std::string&)) {
    DemoTarget target(executable, hidden);
    std::exception_ptr failure;
    bool stopped = false;
    bool bound = false;
    try {
        const HWND window = target.ready(control);
        report("DEMO target-ready");
        checkpoint(control);
        session.bindWindow(window);
        bound = true;
        report("DEMO bound");
        control.waitFor(std::chrono::milliseconds(1500));
        checkpoint(control);
        target.verify();
        if (!session.cmpColor(30, 30, L"ff0000", 1.0) ||
            session.cmpColor(30, 30, L"00ff00", 1.0))
            throw std::runtime_error("Demo target color verification failed");
        report("DEMO color-ok");
        checkpoint(control);
        const auto initial = target.stats(control);
        if (initial.clickDown || initial.clickUp || initial.keyDown || initial.keyUp)
            throw std::runtime_error("Demo target received unexpected input before test");
        target.verify();
        session.moveTo(30, 30);
        checkpoint(control);
        session.leftClick();
        control.waitFor(std::chrono::milliseconds(300));
        checkpoint(control);
        const auto clicked = target.stats(control);
        if (clicked.clickDown != 1 || clicked.clickUp != 1 || clicked.keyDown || clicked.keyUp)
            throw std::runtime_error("Demo target did not receive exactly one click");
        report("DEMO click-ok");
        checkpoint(control);
        target.verify();
        session.keyPress(0x41);
        control.waitFor(std::chrono::milliseconds(300));
        checkpoint(control);
        const auto typed = target.stats(control);
        if (typed.clickDown != 1 || typed.clickUp != 1 || typed.keyDown != 1 || typed.keyUp != 1)
            throw std::runtime_error("Demo target did not receive exactly one A key press");
        report("DEMO key-ok");
    } catch (const Stopped&) {
        stopped = true;
    } catch (...) {
        failure = std::current_exception();
    }
    try {
        session.unbindWindow();
        if (bound) report("DEMO unbound");
    } catch (...) {
        if (!failure) failure = std::current_exception();
        report("ERROR demo cleanup unbind failed");
    }
    try {
        target.close();
        report("DEMO target-closed");
    } catch (...) {
        if (!failure) failure = std::current_exception();
        report("ERROR demo cleanup target close failed");
    }
    if (failure) std::rethrow_exception(failure);
    report(stopped ? "DEMO stopped" : "DEMO pass");
}

} // namespace dm
