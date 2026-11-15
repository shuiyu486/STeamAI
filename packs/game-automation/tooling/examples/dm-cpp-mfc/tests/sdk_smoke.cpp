// 真实 SDK 的最小绑定/输入冒烟：只在显式提供 exact DLL/SHA-256/版本/CLSID 且带
// --sdk-approved --reg-approved --license-stdin 时运行。许可经父进程私有 stdin 管道单行
// LICENSE\t<code>\t<info>(UTF-8) 输入；文档合同要求 Reg 紧跟对象创建、先于 BindWindow，
// Reg 失败即停且不创建目标窗口。目标窗口是本进程自建的唯一 class 普通 Win32 窗口：默认隐藏
// （永不 ShowWindow），或显式 --placement offscreen 屏外显示但不激活。真实 SDK 隐藏目标的后台
// 取色可能如实失败；本冒烟不伪造颜色结果。不做 normal 显示模式回退，不调用
// SetForegroundWindow / SendInput，不向未绑定目标发送动作；失败先清理再以非 0 退出。
// COM 调用在主线程（MTA）完成，目标窗口消息泵在独立 UI 线程运行。
#include "dm_session.h"
#include "license_input.h"

#include <algorithm>
#include <atomic>
#include <chrono>
#include <future>
#include <iostream>
#include <map>
#include <memory>
#include <stdexcept>
#include <string>
#include <thread>

namespace {

constexpr int kClientWidth = 200;  // 客户区左半纯红、右半纯蓝，供 CmpColor 实测。
constexpr int kClientHeight = 150;
constexpr int kClickX = 50;
constexpr int kClickY = 75;
const std::wstring kProbeMatch = L"ff0000";  // 文档 RRGGBB 格式，对应 RGB(0xff,0,0)。
const std::wstring kProbeWrong = L"00ff00";
constexpr double kSim = 0.9;
constexpr int kKeyVk = 0x41;     // 'A'
constexpr int kSettleMs = 1500;  // 文档：绑定后一般需 1–2 秒延时。
constexpr int kWaitMs = 5000;
constexpr int kGraceMs = 300;

void report(const std::string& text) { std::cout << text << std::endl; }

std::string narrow(const std::wstring& text) {
    std::string result;
    for (wchar_t c : text) result.push_back(c < 128 ? static_cast<char>(c) : '?');
    return result;
}

std::string handleText(HWND window) {
    return std::to_string(static_cast<long>(reinterpret_cast<LONG_PTR>(window)));
}

struct TargetState {
    std::atomic<long> mouseMoves{0};
    std::atomic<long> clickDowns{0};
    std::atomic<long> clickUps{0};
    std::atomic<long> keyDowns{0};
    std::atomic<long> keyUps{0};
    std::atomic<bool> stopped{false};
};

TargetState* stateOf(HWND window) {
    return reinterpret_cast<TargetState*>(GetWindowLongPtrW(window, GWLP_USERDATA));
}

void paintClient(HDC dc, const RECT& client) {
    RECT left = client;
    left.right = client.left + (client.right - client.left) / 2;
    RECT right = client;
    right.left = left.right;
    HBRUSH red = CreateSolidBrush(RGB(0xff, 0x00, 0x00));
    HBRUSH blue = CreateSolidBrush(RGB(0x00, 0x00, 0xff));
    if (red && blue) {
        FillRect(dc, &left, red);
        FillRect(dc, &right, blue);
    }
    if (red) DeleteObject(red);
    if (blue) DeleteObject(blue);
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
        if (dc) paintClient(dc, client);
        EndPaint(window, &paint);
        return 0;
    }
    case WM_PRINTCLIENT: {
        RECT client{};
        GetClientRect(window, &client);
        paintClient(reinterpret_cast<HDC>(wParam), client);
        return 0;
    }
    case WM_MOUSEMOVE:
        if (auto* state = stateOf(window)) state->mouseMoves.fetch_add(1);
        return 0;
    case WM_LBUTTONDOWN:
        if (auto* state = stateOf(window)) state->clickDowns.fetch_add(1);
        return 0;
    case WM_LBUTTONUP:
        if (auto* state = stateOf(window)) state->clickUps.fetch_add(1);
        return 0;
    case WM_KEYDOWN:
        if (auto* state = stateOf(window))
            if (wParam == kKeyVk) state->keyDowns.fetch_add(1);
        return 0;
    case WM_KEYUP:
        if (auto* state = stateOf(window))
            if (wParam == kKeyVk) state->keyUps.fetch_add(1);
        return 0;
    case WM_DESTROY:
        PostQuitMessage(0);
        return 0;
    default:
        return DefWindowProcW(window, message, wParam, lParam);
    }
}

void runTarget(std::shared_ptr<TargetState> state, bool offscreen, std::promise<HWND> ready) {
    const std::wstring className = L"DmSdkSmokeTarget_" + std::to_wstring(GetCurrentProcessId());
    const HINSTANCE instance = GetModuleHandleW(nullptr);
    WNDCLASSW registration{};
    registration.style = CS_HREDRAW | CS_VREDRAW;
    registration.lpfnWndProc = targetProc;
    registration.hInstance = instance;
    registration.hCursor = LoadCursorW(nullptr, IDC_ARROW);
    registration.lpszClassName = className.c_str();
    if (!RegisterClassW(&registration)) {
        ready.set_value(nullptr);
        return;
    }
    RECT rect{0, 0, kClientWidth, kClientHeight};
    AdjustWindowRect(&rect, WS_OVERLAPPEDWINDOW, FALSE);
    int x = CW_USEDEFAULT;
    int y = CW_USEDEFAULT;
    if (offscreen) {
        // 完全放到虚拟屏幕右边界之外；显示但不激活。
        x = GetSystemMetrics(SM_XVIRTUALSCREEN) + GetSystemMetrics(SM_CXVIRTUALSCREEN) + 64;
        y = GetSystemMetrics(SM_YVIRTUALSCREEN) + 64;
    }
    // 普通窗口；hidden 布局不加 WS_VISIBLE 且永不 ShowWindow。
    HWND window = CreateWindowExW(WS_EX_NOACTIVATE | WS_EX_TOOLWINDOW, className.c_str(),
                                  L"dm sdk smoke target", WS_OVERLAPPEDWINDOW, x, y,
                                  rect.right - rect.left, rect.bottom - rect.top, nullptr,
                                  nullptr, instance, state.get());
    if (!window) {
        UnregisterClassW(className.c_str(), instance);
        ready.set_value(nullptr);
        return;
    }
    if (offscreen) {
        ShowWindow(window, SW_SHOWNOACTIVATE);
        UpdateWindow(window);
    }
    ready.set_value(window);
    MSG message{};
    while (GetMessageW(&message, nullptr, 0, 0) > 0) {
        TranslateMessage(&message);
        DispatchMessageW(&message);
    }
    state->stopped.store(true);
    UnregisterClassW(className.c_str(), instance);
}

bool waitFor(const std::atomic<long>& counter, long expected, int timeoutMs) {
    const auto deadline = std::chrono::steady_clock::now() + std::chrono::milliseconds(timeoutMs);
    while (std::chrono::steady_clock::now() < deadline) {
        if (counter.load() >= expected) return true;
        std::this_thread::sleep_for(std::chrono::milliseconds(10));
    }
    return counter.load() >= expected;
}

struct Options {
    dm::SdkConfig sdk;
    bool offscreen = false;
};

Options parse(int argc, wchar_t** argv) {
    const std::string usage =
        "Usage: dm_sdk_smoke --sdk-approved --reg-approved --license-stdin --dll PATH --sha256 HASH"
        " --version VERSION --clsid GUID [--placement hidden|offscreen]";
    if (argc < 4 || std::wstring(argv[1]) != L"--sdk-approved" ||
        std::wstring(argv[2]) != L"--reg-approved" || std::wstring(argv[3]) != L"--license-stdin")
        throw std::runtime_error(usage);
    if ((argc - 4) % 2 != 0 || (argc - 4) / 2 < 4) throw std::runtime_error(usage);
    std::map<std::wstring, std::wstring> fields;
    for (int i = 4; i < argc; i += 2) {
        const std::wstring key(argv[i]);
        if (key != L"--dll" && key != L"--sha256" && key != L"--version" && key != L"--clsid" &&
            key != L"--placement")
            throw std::runtime_error("Unknown smoke option");
        if (!fields.emplace(key, argv[i + 1]).second)
            throw std::runtime_error("Duplicate smoke option");
    }
    if (!fields.count(L"--dll") || !fields.count(L"--sha256") || !fields.count(L"--version") ||
        !fields.count(L"--clsid"))
        throw std::runtime_error(usage);
    Options result;
    result.sdk.dll = fields.at(L"--dll");
    const auto hash = fields.at(L"--sha256");
    if (!std::all_of(hash.begin(), hash.end(), [](wchar_t c) { return c >= 0 && c <= 127; }))
        throw std::runtime_error("SHA-256 must be ASCII hexadecimal");
    for (wchar_t c : hash) result.sdk.sha256.push_back(static_cast<char>(c));
    result.sdk.version = fields.at(L"--version");
    result.sdk.clsid = fields.at(L"--clsid");
    if (fields.count(L"--placement")) {
        const auto placement = fields.at(L"--placement");
        if (placement == L"hidden") result.offscreen = false;
        else if (placement == L"offscreen") result.offscreen = true;
        else throw std::runtime_error("--placement must be hidden or offscreen");
    }
    return result;
}

int runScenario(const Options& options) {
    const std::string placement = options.offscreen ? "offscreen" : "hidden";
    // 许可经父进程私有管道单行输入；读取失败不加载 DLL、不创建任何窗口。
    dm::LicenseInput license;
    try {
        license = dm::readLicenseInput();
    } catch (const std::exception& e) {
        report(std::string("SMOKE FAIL license-input ") + e.what());
        report("SMOKE fail");
        return 1;
    }
    // 泵线程持有 state 的 shared_ptr 副本；即使退出超时后线程仍存活，也不会悬空引用。
    std::shared_ptr<TargetState> state;
    std::thread pump;
    HWND window = nullptr;
    bool targetCreated = false;
    bool passed = false;
    bool sessionCreated = false;
    {
        std::unique_ptr<dm::Session> session;
        try {
            session = std::make_unique<dm::Session>(options.sdk);
            sessionCreated = true;
            report("OBS ver ok version=" + narrow(session->version()));
            // 文档合同：Reg 紧跟对象创建、先于 BindWindow；Reg 失败即停，不创建目标窗口。
            session->registerLicense(license.code, license.info);
            license.clear();
            report("OBS license-registered");
            state = std::make_shared<TargetState>();
            std::promise<HWND> readyPromise;
            auto ready = readyPromise.get_future();
            pump = std::thread(runTarget, state, options.offscreen, std::move(readyPromise));
            if (ready.wait_for(std::chrono::seconds(5)) != std::future_status::ready ||
                (window = ready.get()) == nullptr) {
                report("SMOKE FAIL target-window; cannot create own " + placement + " window");
                pump.join();
                return 1;
            }
            targetCreated = true;
            report("SMOKE target-ready placement=" + placement + " hwnd=" + handleText(window));
            session->bindWindow(window);
            report("OBS bind ok business=1 getbind=" + std::to_string(session->boundWindow()));
            std::this_thread::sleep_for(std::chrono::milliseconds(kSettleMs));
            const bool matched = session->cmpColor(kClickX, kClickY, kProbeMatch, kSim);
            report("OBS color match x=" + std::to_string(kClickX) + " y=" + std::to_string(kClickY) +
                   " color=ff0000 business=" + (matched ? "0" : "1") +
                   " matched=" + (matched ? "yes" : "no"));
            const bool wrong = session->cmpColor(kClickX, kClickY, kProbeWrong, kSim);
            report("OBS color mismatch-probe x=" + std::to_string(kClickX) + " y=" +
                   std::to_string(kClickY) + " color=00ff00 business=" + (wrong ? "0" : "1") +
                   " matched=" + (wrong ? "yes" : "no"));
            if (!matched || wrong)
                throw std::runtime_error("capture-not-verified at " + placement + " placement");
            session->moveTo(kClickX, kClickY);
            session->leftClick();
            if (!waitFor(state->clickDowns, 1, kWaitMs) || !waitFor(state->clickUps, 1, kWaitMs))
                throw std::runtime_error("click events not observed within deadline");
            std::this_thread::sleep_for(std::chrono::milliseconds(kGraceMs));
            if (state->clickDowns.load() != 1 || state->clickUps.load() != 1)
                throw std::runtime_error("click count not exactly 1: down=" +
                                         std::to_string(state->clickDowns.load()) + " up=" +
                                         std::to_string(state->clickUps.load()));
            report("OBS click down=1 up=1 mousemoves=" + std::to_string(state->mouseMoves.load()));
            session->keyPress(kKeyVk);
            if (!waitFor(state->keyDowns, 1, kWaitMs) || !waitFor(state->keyUps, 1, kWaitMs))
                throw std::runtime_error("key events not observed within deadline");
            std::this_thread::sleep_for(std::chrono::milliseconds(kGraceMs));
            if (state->keyDowns.load() != 1 || state->keyUps.load() != 1)
                throw std::runtime_error("key count not exactly 1: down=" +
                                         std::to_string(state->keyDowns.load()) + " up=" +
                                         std::to_string(state->keyUps.load()));
            report("OBS key vk=0x41 down=1 up=1");
            // 显式解绑并核对业务成功与绑定解除；这一步成功才构成本次冒烟的清理证据。
            session->unbindWindow();
            report("OBS unbind ok business=1 getbind=0");
            passed = true;
        } catch (const std::exception& e) {
            report("SMOKE FAIL " + std::string(e.what()));
            if (session) {
                try {
                    session->unbindWindow();
                    report("SMOKE cleanup-unbind-ok");
                } catch (const std::exception& cleanup) {
                    report(std::string("SMOKE cleanup-unbind-failed ") + cleanup.what());
                }
            }
        }
    }
    if (sessionCreated) report("SMOKE released");
    license.clear(); // Reg 成功路径已清理；失败路径在此兜底，密钥不随异常残留。

    if (targetCreated) {
        PostMessageW(window, WM_CLOSE, 0, 0);
        const auto deadline = std::chrono::steady_clock::now() + std::chrono::seconds(5);
        while (!state->stopped.load() && std::chrono::steady_clock::now() < deadline)
            std::this_thread::sleep_for(std::chrono::milliseconds(10));
        if (state->stopped.load()) {
            pump.join();
            report("SMOKE target-closed");
        } else {
            // 泵线程未按期退出：本次判定失败并以非 0 退出；线程持有 state 的 shared_ptr，
            // 进程结束时随之结束，不强杀线程，也不留下对栈的悬空引用。
            pump.detach();
            passed = false;
            report("SMOKE target-pump-timeout");
        }
    }
    report(passed ? "SMOKE pass" : "SMOKE fail");
    return passed ? 0 : 1;
}

} // namespace

int wmain(int argc, wchar_t** argv) {
    try {
        return runScenario(parse(argc, argv));
    } catch (const std::exception& e) {
        report(std::string("ERROR ") + e.what());
        return 2;
    } catch (...) {
        report("ERROR unexpected smoke failure");
        return 2;
    }
}
