// 独立 Win32 演示目标（demo_target）：只自建并持有本进程的唯一窗口，不读用户窗口、
// 不调用大漠 SDK。默认可见（SW_SHOWNOACTIVATE，不抢焦点；用户可手动打开观察、点 X
// 正常关闭）；维护测试显式 --hidden：与 sdk_smoke 的 hidden 布局一致，不加 WS_VISIBLE、
// 永不 ShowWindow、不激活，且要求 stdin 是父进程私有管道（否则拒绝启动，避免留下
// 用户无法关闭的隐藏窗口）。
// 管道协议（对齐 dm_console WorkerProcess 客户端；stdin 是管道即协议模式，行命令，
// \n 结束）：启动第一行 stdout 为
//   TARGET ready <unsigned decimal HWND> <decimal PID>
// 命令仅三个：
//   stats -> TARGET stats <mousemoves> <clickDowns> <clickUps> <keyDowns> <keyUps>
//            （十进制空格分隔；key 计数仅 VK 0x41 'A'）
//   page  -> TARGET page <current>   （当前页，1 起；经典模式恒为 1）
//   close -> 不回包；正常关自有窗口，退出码 0
// stdin EOF 等同 close（父进程正常收尾或异常死亡都让目标退出，退出码 0）。无效或
// 超长命令按协议错误停止：stdout 不再出现 TARGET 行、错误信息走 stderr、退出码非 0
// （同 dm_worker 的 fail-closed 约定）；启动失败不出 ready、退出非 0。手动模式
// （stdin 非管道且未 --hidden）不走协议、不写 stdout。不做网络、不做自动发现、
// 不接收任何 hwnd 输入。
// 绘制复用 tests/sdk_smoke.cpp 的客户区语义：上方 200x150 区域左半纯红 RGB(255,0,0)、
// 右半纯蓝 RGB(0,0,255)；主线程红色探针 (30,30) 在该区域内。计数状态条固定在底部
// （y>=150），不覆盖任何色块点。输入计数与 sdk_smoke 完全一致：WM_LBUTTONDOWN/UP、
// WM_KEYDOWN/UP（仅 VK 0x41）、WM_MOUSEMOVE。
// --pages N（2..8）：页签布局，是"打开/切页"的无消耗、可重复等价目标。客户区顶部增加
// 28px 页签条（N 个页签，显示页号，当前页黄底黑框），下方 150px 内容区按当前页显示
// 左右两色（第 1 页与经典红/蓝相同），底部计数条首行增加 page=k/N。页签区内按下
// （WM_LBUTTONDOWN 的客户区坐标）切换当前页，其它区域只计数；重复点同一页不变，不消耗
// 任何状态。不带 --pages 的经典模式在客户区尺寸、绘制、计数与协议行上保持不变。
// 入口 wWinMain（GUI 子系统）。构建已由 CMake 接线：add_executable(dm_demo_target
// WIN32 src/demo_target.cpp)，链接 user32/gdi32/shell32。
#include <windows.h>

#include <windowsx.h>  // GET_X_LPARAM / GET_Y_LPARAM

#include <atomic>
#include <cstdlib>  // MSVC CRT 的 __argc / __wargv（wWinMain 无 argc/argv 形参）。
#include <stdexcept>
#include <string>

namespace {

constexpr int kClientWidth = 200;   // 与 sdk_smoke 相同的色块区宽度。
constexpr int kColorHeight = 150;   // 与 sdk_smoke 相同：左半纯红、右半纯蓝。
constexpr int kStatusHeight = 40;   // 底部两行计数状态条。
constexpr int kTabHeight = 28;      // --pages：顶部页签条高度。
constexpr int kClientHeight = kColorHeight + kStatusHeight;
constexpr int kKeyVk = 0x41;        // 'A'：与 sdk_smoke 相同，只统计该键。
constexpr size_t kMaxCommandLength = 64;
constexpr int kMaxPages = 8;
// 各页内容区配色（左半 | 右半）；第 1 页与经典布局相同。
const COLORREF kPageColors[kMaxPages][2] = {
    {RGB(0xff, 0x00, 0x00), RGB(0x00, 0x00, 0xff)}, {RGB(0x00, 0xc8, 0x00), RGB(0xff, 0xff, 0x00)},
    {RGB(0xff, 0x00, 0xff), RGB(0x00, 0xff, 0xff)}, {RGB(0xff, 0xa5, 0x00), RGB(0x80, 0x00, 0x80)},
    {RGB(0x00, 0x80, 0x80), RGB(0xff, 0xc0, 0x80)}, {RGB(0x80, 0x80, 0x00), RGB(0x00, 0x00, 0x80)},
    {RGB(0x80, 0x00, 0x00), RGB(0x00, 0x80, 0x00)}, {RGB(0xc0, 0xc0, 0x00), RGB(0x00, 0xc0, 0xc0)},
};

struct TargetState {
    std::atomic<long> mouseMoves{0};
    std::atomic<long> clickDowns{0};
    std::atomic<long> clickUps{0};
    std::atomic<long> keyDowns{0};
    std::atomic<long> keyUps{0};
    std::atomic<long> page{1};   // --pages：当前页（1 起）；经典模式恒为 1。
    int pages = 0;               // 0 = 经典布局。构造后只读。
    bool protocolError = false;  // 仅消息循环线程访问。
};

TargetState* stateOf(HWND window) {
    return reinterpret_cast<TargetState*>(GetWindowLongPtrW(window, GWLP_USERDATA));
}

std::string handleText(HWND window) {
    // 无符号十进制：HWND 是句柄值不是有符号数，客户端按非负数解析。
    return std::to_string(static_cast<unsigned long>(reinterpret_cast<ULONG_PTR>(window)));
}

std::string readyText(HWND window) {
    return "TARGET ready " + handleText(window) + " " + std::to_string(GetCurrentProcessId());
}

int tabWidth(int pages) { return kClientWidth / pages; }

RECT tabRect(int pages, int index) {
    // 页签在各自等宽格内留 2px 边距；格间空隙与上下边距不响应切换。
    RECT tab{};
    tab.left = index * tabWidth(pages) + 2;
    tab.top = 2;
    tab.right = (index + 1) * tabWidth(pages) - 2;
    tab.bottom = kTabHeight - 2;
    return tab;
}

int contentTop(const TargetState& state) { return state.pages ? kTabHeight : 0; }

// --pages：页签区内的按下切换当前页；其它区域只计数。同一页重复按下不变。
void switchPage(TargetState& state, int x, int y) {
    const POINT point{x, y};
    for (int index = 0; index < state.pages; ++index) {
        RECT tab = tabRect(state.pages, index);
        if (PtInRect(&tab, point)) {
            state.page.store(index + 1);
            return;
        }
    }
}

void paintTabs(HDC dc, const TargetState& state) {
    RECT strip{0, 0, kClientWidth, kTabHeight};
    HBRUSH background = CreateSolidBrush(RGB(0xe6, 0xe6, 0xe6));
    if (background) {
        FillRect(dc, &strip, background);
        DeleteObject(background);
    }
    const int previousMode = SetBkMode(dc, TRANSPARENT);
    HGDIOBJ previousFont = SelectObject(dc, GetStockObject(DEFAULT_GUI_FONT));
    for (int index = 0; index < state.pages; ++index) {
        const bool active = index + 1 == state.page.load();
        RECT tab = tabRect(state.pages, index);
        HBRUSH fill = CreateSolidBrush(active ? RGB(0xff, 0xff, 0x00) : RGB(0xc8, 0xc8, 0xc8));
        HBRUSH border = CreateSolidBrush(active ? RGB(0x00, 0x00, 0x00) : RGB(0x78, 0x78, 0x78));
        if (fill) {
            FillRect(dc, &tab, fill);
            DeleteObject(fill);
        }
        if (border) {
            FrameRect(dc, &tab, border);
            DeleteObject(border);
        }
        // 页号文字同时给页签提供空间纹理（图像定位模板不接纳纯色块）。
        SetTextColor(dc, RGB(0x00, 0x00, 0x00));
        const std::wstring text = std::to_wstring(index + 1);
        DrawTextW(dc, text.c_str(), -1, &tab, DT_SINGLELINE | DT_CENTER | DT_VCENTER | DT_NOPREFIX);
    }
    if (previousFont) SelectObject(dc, previousFont);
    SetBkMode(dc, previousMode);
}

void paintColorArea(HDC dc, const RECT& area, const TargetState& state) {
    // 与 sdk_smoke.paintClient 相同的左右两色；--pages 时取当前页配色（第 1 页即红/蓝）。
    RECT left = area;
    left.right = area.left + (area.right - area.left) / 2;
    RECT right = area;
    right.left = left.right;
    HBRUSH leftBrush = CreateSolidBrush(kPageColors[state.page.load() - 1][0]);
    HBRUSH rightBrush = CreateSolidBrush(kPageColors[state.page.load() - 1][1]);
    if (leftBrush && rightBrush) {
        FillRect(dc, &left, leftBrush);
        FillRect(dc, &right, rightBrush);
    }
    if (leftBrush) DeleteObject(leftBrush);
    if (rightBrush) DeleteObject(rightBrush);
}

void paintStatusStrip(HDC dc, const RECT& strip, const TargetState& state) {
    HBRUSH background = CreateSolidBrush(RGB(0xf0, 0xf0, 0xf0));
    if (background) {
        FillRect(dc, &strip, background);
        DeleteObject(background);
    }
    const int previousMode = SetBkMode(dc, TRANSPARENT);
    const COLORREF previousColor = SetTextColor(dc, RGB(0x00, 0x00, 0x00));
    HGDIOBJ previousFont = SelectObject(dc, GetStockObject(DEFAULT_GUI_FONT));
    RECT firstLine = strip;
    firstLine.bottom = (strip.top + strip.bottom) / 2;
    RECT secondLine = strip;
    secondLine.top = firstLine.bottom;
    std::wstring clickText = L"click down=" + std::to_wstring(state.clickDowns.load()) +
                             L" up=" + std::to_wstring(state.clickUps.load());
    if (state.pages)
        clickText = L"page=" + std::to_wstring(state.page.load()) + L"/" +
                    std::to_wstring(state.pages) + L"  " + clickText;
    const std::wstring keyText = L"A down=" + std::to_wstring(state.keyDowns.load()) +
                                 L" up=" + std::to_wstring(state.keyUps.load()) +
                                 L" move=" + std::to_wstring(state.mouseMoves.load());
    DrawTextW(dc, clickText.c_str(), -1, &firstLine,
              DT_SINGLELINE | DT_CENTER | DT_VCENTER | DT_NOPREFIX);
    DrawTextW(dc, keyText.c_str(), -1, &secondLine,
              DT_SINGLELINE | DT_CENTER | DT_VCENTER | DT_NOPREFIX);
    if (previousFont) SelectObject(dc, previousFont);
    SetTextColor(dc, previousColor);
    SetBkMode(dc, previousMode);
}

void paintTarget(HDC dc, const RECT& client, const TargetState& state) {
    // 自上而下：页签条（仅 --pages）→ 150px 内容区 → 底部计数状态条。
    const int top = contentTop(state);
    if (state.pages) paintTabs(dc, state);
    RECT colorArea = client;
    colorArea.top = client.top + top;
    colorArea.bottom = client.top + top + kColorHeight;
    RECT statusStrip = client;
    statusStrip.top = client.top + top + kColorHeight;
    paintColorArea(dc, colorArea, state);
    paintStatusStrip(dc, statusStrip, state);
}

LRESULT CALLBACK targetProc(HWND window, UINT message, WPARAM wParam, LPARAM lParam) {
    switch (message) {
    case WM_CREATE: {
        auto* creation = reinterpret_cast<CREATESTRUCTW*>(lParam);
        SetWindowLongPtrW(window, GWLP_USERDATA,
                          reinterpret_cast<LONG_PTR>(creation->lpCreateParams));
        return 0;
    }
    case WM_ERASEBKGND:
        return 1; // 背景由客户区整幅重画。
    case WM_PAINT: {
        PAINTSTRUCT paint{};
        RECT client{};
        GetClientRect(window, &client);
        HDC dc = BeginPaint(window, &paint);
        if (dc) {
            if (auto* state = stateOf(window)) paintTarget(dc, client, *state);
        }
        EndPaint(window, &paint);
        return 0;
    }
    case WM_PRINTCLIENT: {
        RECT client{};
        GetClientRect(window, &client);
        if (auto* state = stateOf(window)) paintTarget(reinterpret_cast<HDC>(wParam), client, *state);
        return 0;
    }
    case WM_MOUSEMOVE:
        if (auto* state = stateOf(window)) state->mouseMoves.fetch_add(1);
        InvalidateRect(window, nullptr, FALSE);
        return 0;
    case WM_LBUTTONDOWN:
        if (auto* state = stateOf(window)) {
            state->clickDowns.fetch_add(1);
            if (state->pages) switchPage(*state, GET_X_LPARAM(lParam), GET_Y_LPARAM(lParam));
        }
        InvalidateRect(window, nullptr, FALSE);
        return 0;
    case WM_LBUTTONUP:
        if (auto* state = stateOf(window)) state->clickUps.fetch_add(1);
        InvalidateRect(window, nullptr, FALSE);
        return 0;
    case WM_KEYDOWN:
        if (auto* state = stateOf(window))
            if (wParam == kKeyVk) {
                state->keyDowns.fetch_add(1);
                InvalidateRect(window, nullptr, FALSE);
            }
        return 0;
    case WM_KEYUP:
        if (auto* state = stateOf(window))
            if (wParam == kKeyVk) {
                state->keyUps.fetch_add(1);
                InvalidateRect(window, nullptr, FALSE);
            }
        return 0;
    case WM_DESTROY:
        PostQuitMessage(0);
        return 0;
    default:
        return DefWindowProcW(window, message, wParam, lParam);
    }
}

// 输出尽力而为：父进程未接读端或手动启动无句柄时忽略失败，不影响窗口生命周期。
// stdout 只出现 TARGET 行；错误细节一律走 stderr。
void sendLine(HANDLE output, const std::string& text) {
    if (!output || output == INVALID_HANDLE_VALUE) return;
    const std::string line = text + "\n";
    DWORD written = 0;
    WriteFile(output, line.data(), static_cast<DWORD>(line.size()), &written, nullptr);
}

void logError(const std::string& text) { sendLine(GetStdHandle(STD_ERROR_HANDLE), "ERROR " + text); }

enum class Command { keepGoing, stop };

Command dispatchCommand(HANDLE output, TargetState& state, const std::string& line) {
    if (line == "stats") {
        sendLine(output, "TARGET stats " + std::to_string(state.mouseMoves.load()) + " " +
                         std::to_string(state.clickDowns.load()) + " " +
                         std::to_string(state.clickUps.load()) + " " +
                         std::to_string(state.keyDowns.load()) + " " +
                         std::to_string(state.keyUps.load()));
        return Command::keepGoing;
    }
    if (line == "close") return Command::stop;  // 不回包；正常关窗，退出码 0。
    if (line == "page") {
        sendLine(output, "TARGET page " + std::to_string(state.page.load()));
        return Command::keepGoing;
    }
    state.protocolError = true;
    logError("invalid command; stopping");
    return Command::stop;
}

// 逐字节喂入一条行命令；返回 false 表示需要停止（close 或协议错误，同 dm_worker）。
class CommandReader {
public:
    CommandReader(HANDLE output, TargetState& state) : output_(output), state_(state) {}

    bool feed(char c) {
        if (c == '\r') return true;
        if (c == '\n') {
            const Command result = dispatchCommand(output_, state_, line_);
            line_.clear();
            return result == Command::keepGoing;
        }
        if (line_.size() >= kMaxCommandLength) {
            state_.protocolError = true;
            logError("command too long; stopping");
            return false;
        }
        line_ += c;
        return true;
    }

private:
    HANDLE output_;
    TargetState& state_;
    std::string line_;
};

// 单线程消息循环：有界 10ms 消息等待 + PeekNamedPipe 轮询（同 dm_worker 的 10ms 节拍；
// 匿名管道句柄不保证 MsgWaitForMultipleObjects 的“数据可读”唤醒，不能挂在其上等待）。
// 每次唤醒先排空窗口消息再读管道，保证先入队的输入消息先于后到的命令被计数。
int runLoop(HWND window, TargetState& state, bool protocol, HANDLE input, HANDLE output) {
    CommandReader reader(output, state);
    bool watchingPipe = protocol;
    bool closeRequested = false;
    const auto requestClose = [&] {
        if (closeRequested) return;
        closeRequested = true;
        PostMessageW(window, WM_CLOSE, 0, 0);  // 正常关闭：DefWindowProc -> DestroyWindow。
    };
    for (;;) {
        MsgWaitForMultipleObjects(0, nullptr, FALSE, 10, QS_ALLINPUT);
        MSG message;
        while (PeekMessageW(&message, nullptr, 0, 0, PM_REMOVE)) {
            if (message.message == WM_QUIT)
                return state.protocolError ? 2 : static_cast<int>(message.wParam);
            TranslateMessage(&message);
            DispatchMessageW(&message);
        }
        if (!watchingPipe) continue;
        // 只读 Peek 已确认存在的字节：匿名管道上请求超过现存字节数会阻塞等待读满。
        // EOF / close / 协议错误即请求正常关闭。
        bool stopReading = false;
        while (!stopReading) {
            DWORD available = 0;
            if (!PeekNamedPipe(input, nullptr, 0, nullptr, &available, nullptr)) {
                watchingPipe = false;  // stdin EOF：请求退出后不再轮询该句柄。
                requestClose();
                break;
            }
            if (available == 0) break;
            char chunk[64];
            const DWORD limit = static_cast<DWORD>(sizeof(chunk));
            const DWORD toRead = available < limit ? available : limit;
            DWORD count = 0;
            if (!ReadFile(input, chunk, toRead, &count, nullptr) || count == 0) {
                watchingPipe = false;
                requestClose();
                break;
            }
            for (DWORD i = 0; i < count; ++i) {
                if (!reader.feed(chunk[i])) {
                    stopReading = true;  // close 或协议错误：忽略后续输入并关闭。
                    break;
                }
            }
        }
        if (stopReading) {
            watchingPipe = false;
            requestClose();
        }
    }
}

struct Options {
    bool hidden = false;
    int pages = 0;  // 0 = 经典单页布局；--pages N 为 2..kMaxPages 的页签布局。
};

Options parse(int argc, wchar_t** argv) {
    const char* const usage = "Usage: dm_demo_target [--hidden] [--pages N]";
    Options result;
    for (int i = 1; i < argc; ++i) {
        const std::wstring argument(argv[i]);
        if (argument == L"--hidden") {
            if (result.hidden) throw std::runtime_error("Duplicate --hidden");
            result.hidden = true;
        } else if (argument == L"--pages") {
            // 只接受 1..2 位十进制且落在 2..kMaxPages：非法值在创建窗口前拒绝（退出码 2）。
            if (result.pages) throw std::runtime_error("Duplicate --pages");
            if (++i == argc) throw std::runtime_error("Missing value for --pages");
            const std::wstring value(argv[i]);
            int pages = 0;
            if (value.empty() || value.size() > 2) throw std::runtime_error(usage);
            for (wchar_t c : value) {
                if (c < L'0' || c > L'9') throw std::runtime_error(usage);
                pages = pages * 10 + (c - L'0');
            }
            if (pages < 2 || pages > kMaxPages) throw std::runtime_error(usage);
            result.pages = pages;
        } else {
            throw std::runtime_error(usage);
        }
    }
    return result;
}

int run(const Options& options) {
    const HANDLE input = GetStdHandle(STD_INPUT_HANDLE);
    const HANDLE output = GetStdHandle(STD_OUTPUT_HANDLE);
    const bool pipedStdin = input != nullptr && input != INVALID_HANDLE_VALUE &&
                            GetFileType(input) == FILE_TYPE_PIPE;
    if (options.hidden && !pipedStdin) {
        // --hidden 只属于父进程管道测试；无管道的隐藏窗口用户无法关闭。
        logError("--hidden requires a parent stdin pipe");
        return 2;
    }
    TargetState state;  // 窗口经 GWLP_USERDATA 引用它；窗口销毁先于本函数返回。
    state.pages = options.pages;
    const std::wstring className = L"DmDemoTarget_" + std::to_wstring(GetCurrentProcessId());
    const HINSTANCE instance = GetModuleHandleW(nullptr);
    WNDCLASSW registration{};
    registration.style = CS_HREDRAW | CS_VREDRAW;
    registration.lpfnWndProc = targetProc;
    registration.hInstance = instance;
    registration.hCursor = LoadCursorW(nullptr, IDC_ARROW);
    registration.lpszClassName = className.c_str();
    if (!RegisterClassW(&registration)) {
        logError("cannot register window class");
        return 1;  // 未出 ready，退出非 0。
    }
    RECT rect{0, 0, kClientWidth, kClientHeight + (options.pages ? kTabHeight : 0)};
    AdjustWindowRect(&rect, WS_OVERLAPPEDWINDOW, FALSE);
    const DWORD exStyle = options.hidden ? WS_EX_NOACTIVATE | WS_EX_TOOLWINDOW : 0;
    HWND window = CreateWindowExW(exStyle, className.c_str(), L"dm demo target",
                                  WS_OVERLAPPEDWINDOW, CW_USEDEFAULT, CW_USEDEFAULT,
                                  rect.right - rect.left, rect.bottom - rect.top, nullptr,
                                  nullptr, instance, &state);
    if (!window) {
        UnregisterClassW(className.c_str(), instance);
        logError("cannot create target window");
        return 1;  // 未出 ready，退出非 0。
    }
    SetWindowTextW(window, (L"dm demo target pid=" + std::to_wstring(GetCurrentProcessId()) +
                            L" hwnd=" + std::to_wstring(static_cast<unsigned long>(
                                             reinterpret_cast<ULONG_PTR>(window))))
                                .c_str());
    if (!options.hidden) {
        ShowWindow(window, SW_SHOWNOACTIVATE);  // 可见但不抢焦点。
        UpdateWindow(window);
    }
    // --hidden：不加 WS_VISIBLE 且永不 ShowWindow，与 sdk_smoke hidden 布局一致。
    if (pipedStdin) sendLine(output, readyText(window));
    const int code = runLoop(window, state, pipedStdin, input, output);
    UnregisterClassW(className.c_str(), instance);
    return code;
}

} // namespace

int APIENTRY wWinMain(HINSTANCE, HINSTANCE, PWSTR, int) {
    try {
        return run(parse(__argc, __wargv));
    } catch (const std::exception& e) {
        logError(e.what());
        return 2;
    } catch (...) {
        logError("unexpected demo target failure");
        return 2;
    }
}
