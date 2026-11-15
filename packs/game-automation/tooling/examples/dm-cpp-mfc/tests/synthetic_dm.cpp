#include <windows.h>
#include <oaidl.h>
#include <algorithm>
#include <atomic>
#include <cstdlib>
#include <cstring>
#include <string>

namespace {
// 仅用于本工程测试的 COM 类，与大漠真实 CLSID 无关。
const CLSID fixtureId = {0x9f174d53, 0x4c94, 0x4710, {0x99, 0x04, 0x10, 0x01, 0x20, 0x02, 0x30, 0x03}};
std::atomic<long> objects{0};
std::atomic<long> factories{0};

void event(const char* line) {
    DWORD written = 0;
    WriteFile(GetStdHandle(STD_OUTPUT_HANDLE), line, static_cast<DWORD>(std::strlen(line)), &written, nullptr);
}

void eventLine(const std::string& line) {
    event((line + "\n").c_str());
}

std::wstring mode() {
    wchar_t value[64]{};
    GetEnvironmentVariableW(L"DM_TEST_BEHAVIOR", value, 64);
    return value;
}

// 取逻辑第 index 个参数；DISPPARAMS 的 rgvarg 是反序存放。
VARIANT* argOf(DISPPARAMS* args, UINT index) {
    if (index >= args->cArgs) return nullptr;
    return &args->rgvarg[args->cArgs - 1 - index];
}

bool isInteger(const VARIANT* value) { return value && value->vt == VT_I4; }
bool isReal(const VARIANT* value) { return value && value->vt == VT_R8; }
bool isText(const VARIANT* value) { return value && value->vt == VT_BSTR; }
std::wstring textOf(const VARIANT* value) {
    return value && value->bstrVal ? std::wstring(value->bstrVal) : std::wstring();
}

// 只支持文档 RRGGBB 单色形态；本工程探测只用该形态，多色/偏色按不匹配处理。
bool parseColor(const std::wstring& text, COLORREF* out) {
    if (text.size() != 6) return false;
    BYTE parts[3]{};
    for (int channel = 0; channel < 3; ++channel)
        for (int digit = 0; digit < 2; ++digit) {
            wchar_t c = text[channel * 2 + digit];
            int value = c >= L'0' && c <= L'9' ? c - L'0'
                        : c >= L'a' && c <= L'f' ? c - L'a' + 10
                        : c >= L'A' && c <= L'F' ? c - L'A' + 10
                        : -1;
            if (value < 0) return false;
            parts[channel] = static_cast<BYTE>(parts[channel] * 16 + value);
        }
    *out = RGB(parts[0], parts[1], parts[2]);
    return true;
}

// 合成 gdi 取色：请目标窗口把客户区画入内存 DC（隐藏窗口同样有效），再取点按 sim 比较。
bool sampleMatches(HWND target, long x, long y, COLORREF expected, double sim) {
    RECT client{};
    if (!GetClientRect(target, &client) || client.right <= 0 || client.bottom <= 0) return false;
    const int width = client.right - client.left;
    const int height = client.bottom - client.top;
    if (x < 0 || y < 0 || x >= width || y >= height) return false;
    DWORD owner = 0;
    GetWindowThreadProcessId(target, &owner);
    if (owner != GetCurrentProcessId()) {
        // 跨进程不能把本进程内存 DC 经 WM_PRINTCLIENT 当作有效画布传给目标。
        // 此分支只模拟固定红蓝测试图，验证类型/参数/控制流程，不声称验证真实截图。
        const COLORREF simulated = x < width / 2 ? RGB(255, 0, 0) : RGB(0, 0, 255);
        return simulated == expected;
    }
    HDC screen = GetDC(nullptr);
    if (!screen) return false;
    HDC memory = CreateCompatibleDC(screen);
    HBITMAP surface = CreateCompatibleBitmap(screen, width, height);
    bool matches = false;
    if (memory && surface) {
        HGDIOBJ previous = SelectObject(memory, surface);
        SendMessageW(target, WM_PRINTCLIENT, reinterpret_cast<WPARAM>(memory), PRF_CLIENT);
        const COLORREF actual = GetPixel(memory, static_cast<int>(x), static_cast<int>(y));
        SelectObject(memory, previous);
        if (actual != CLR_INVALID) {
            const int tolerance = static_cast<int>((1.0 - sim) * 255.0 + 0.5);
            const int diff = std::max({std::abs(static_cast<int>(GetRValue(actual)) -
                                                static_cast<int>(GetRValue(expected))),
                                       std::abs(static_cast<int>(GetGValue(actual)) -
                                                static_cast<int>(GetGValue(expected))),
                                       std::abs(static_cast<int>(GetBValue(actual)) -
                                                static_cast<int>(GetBValue(expected)))});
            matches = diff <= tolerance;
        }
    }
    if (surface) DeleteObject(surface);
    if (memory) DeleteDC(memory);
    ReleaseDC(nullptr, screen);
    return matches;
}

class Object final : public IDispatch {
public:
    Object() : thread_(GetCurrentThreadId()) { ++objects; event("FIXTURE create\n"); }
    HRESULT STDMETHODCALLTYPE QueryInterface(REFIID iid, void** out) override {
        if (!out) return E_POINTER;
        *out = nullptr;
        if (iid != IID_IUnknown && iid != IID_IDispatch) return E_NOINTERFACE;
        *out = static_cast<IDispatch*>(this); AddRef(); return S_OK;
    }
    ULONG STDMETHODCALLTYPE AddRef() override { return ++refs_; }
    ULONG STDMETHODCALLTYPE Release() override {
        const auto left = --refs_;
        if (!left) {
            event(GetCurrentThreadId() == thread_ ? "FIXTURE release-same-thread\n" : "FIXTURE wrong-thread\n");
            --objects; delete this;
        }
        return left;
    }
    HRESULT STDMETHODCALLTYPE GetTypeInfoCount(UINT* count) override { *count = 0; return S_OK; }
    HRESULT STDMETHODCALLTYPE GetTypeInfo(UINT, LCID, ITypeInfo**) override { return E_NOTIMPL; }
    HRESULT STDMETHODCALLTYPE GetIDsOfNames(REFIID, LPOLESTR* names, UINT count, LCID, DISPID* ids) override {
        if (count != 1 || !names || !names[0]) return DISP_E_UNKNOWNNAME;
        const std::wstring name(names[0]);
        static const struct { const wchar_t* name; DISPID id; } table[] = {
            {L"Ver", 1},          {L"BindWindow", 2},   {L"GetBindWindow", 3}, {L"UnBindWindow", 5},
            {L"CmpColor", 6},     {L"MoveTo", 7},       {L"LeftClick", 8},     {L"KeyPress", 9},
            {L"GetLastError", 10}, {L"Reg", 11},
        };
        for (const auto& entry : table)
            if (name == entry.name) { ids[0] = entry.id; return S_OK; }
        return DISP_E_UNKNOWNNAME;
    }
    HRESULT STDMETHODCALLTYPE Invoke(DISPID id, REFIID, LCID, WORD flags, DISPPARAMS* args,
                                     VARIANT* result, EXCEPINFO*, UINT*) override {
        ULONG_PTR context = 0;
        if (GetCurrentThreadId() != thread_ || FAILED(CoGetContextToken(&context))) return E_UNEXPECTED;
        if (flags != DISPATCH_METHOD) return DISP_E_MEMBERNOTFOUND;
        switch (id) {
        case 1: return ver(args, result);
        case 2: return bindWindow(args, result);
        case 3: return getBindWindow(args, result);
        case 5: return unbindWindow(args, result);
        case 6: return cmpColor(args, result);
        case 7: return moveTo(args, result);
        case 8: return leftClick(args, result);
        case 9: return keyPress(args, result);
        case 10: return getLastError(args, result);
        case 11: return reg(args, result);
        default: return DISP_E_MEMBERNOTFOUND;
        }
    }

private:
    HRESULT ver(DISPPARAMS* args, VARIANT* result) {
        if (args->cArgs != 0) return DISP_E_BADPARAMCOUNT;
        const auto behavior = mode();
        if (behavior == L"slow") { event("FIXTURE call-blocked\n"); Sleep(700); }
        // 供界面侧覆盖“停止超过 2 秒如实提示”分支；阻塞长于该时限。
        if (behavior == L"very-slow") { event("FIXTURE call-blocked\n"); Sleep(3000); }
        if (behavior == L"hresult") return E_FAIL;
        if (behavior == L"wrong-type") { result->vt = VT_I4; result->lVal = 1; return S_OK; }
        result->vt = VT_BSTR;
        result->bstrVal = SysAllocString(behavior == L"wrong-version" ? L"fixture-other" : L"fixture-1");
        return result->bstrVal ? S_OK : E_OUTOFMEMORY;
    }
    // 合成 Reg：严格 2 个 VT_BSTR 参数(code, info)；只接受公开的 fixture 测试凭据，
    // 成功输出不含凭据的 FIXTURE licensed 并置 licensed_，凭据错误返回 4(注册码错误)。
    // 与真实大漠一致：凭据原文绝不回显到输出。
    HRESULT reg(DISPPARAMS* args, VARIANT* result) {
        if (args->cArgs != 2) return DISP_E_BADPARAMCOUNT;
        VARIANT* code = argOf(args, 0);
        VARIANT* info = argOf(args, 1);
        if (!isText(code) || !isText(info)) return DISP_E_TYPEMISMATCH;
        result->vt = VT_I4;
        if (textOf(code) == L"fixture-license" && textOf(info) == L"demo") {
            licensed_ = true;
            eventLine("FIXTURE licensed");
            result->lVal = 1;
            return S_OK;
        }
        result->lVal = 4;
        return S_OK;
    }
    HRESULT bindWindow(DISPPARAMS* args, VARIANT* result) {
        result->vt = VT_I4;
        result->lVal = 0;
        // 模拟真实合同：未许可注册的绑定属收费功能，业务返回 0 且 GetLastError=-1。
        if (!licensed_) { lastError_ = -1; eventLine("FIXTURE bind-unlicensed"); return S_OK; }
        VARIANT* handle = argOf(args, 0);
        VARIANT* display = argOf(args, 1);
        VARIANT* mouse = argOf(args, 2);
        VARIANT* keypad = argOf(args, 3);
        VARIANT* bindMode = argOf(args, 4);
        if (args->cArgs != 5 || !isInteger(handle) || !isText(display) || !isText(mouse) ||
            !isText(keypad) || !isInteger(bindMode)) {
            lastError_ = -44;
            eventLine("FIXTURE bad-args BindWindow");
            return S_OK;
        }
        if (textOf(display) != L"gdi" || textOf(mouse) != L"windows" ||
            textOf(keypad) != L"windows" || bindMode->lVal != 0) {
            lastError_ = -44;
            eventLine("FIXTURE bind-mode-rejected");
            return S_OK;
        }
        if (handle->lVal == 0) { lastError_ = -18; eventLine("FIXTURE bind-invalid-hwnd"); return S_OK; }
        if (mode() == L"bind-fail") { lastError_ = -44; eventLine("FIXTURE bind-refused"); return S_OK; }
        bound_ = handle->lVal;
        eventLine("FIXTURE bound");
        result->lVal = 1;
        return S_OK;
    }
    HRESULT getBindWindow(DISPPARAMS* args, VARIANT* result) {
        if (args->cArgs != 0) return DISP_E_BADPARAMCOUNT;
        result->vt = VT_I4;
        long value = bound_;
        // 模拟“BindWindow 业务成功但 GetBindWindow 核对不一致”，
        // 检验先登记后核对的实现仍会解绑这个已生效的绑定。
        if (bound_ != 0 && mode() == L"bind-verify-mismatch") value = bound_ + 1;
        result->lVal = value;
        return S_OK;
    }
    HRESULT unbindWindow(DISPPARAMS* args, VARIANT* result) {
        if (args->cArgs != 0) return DISP_E_BADPARAMCOUNT;
        result->vt = VT_I4;
        result->lVal = 0;
        if (!bound_) return S_OK; // 未绑定解绑按文档业务 0。
        if (mode() == L"unbind-fail") { lastError_ = -18; eventLine("FIXTURE unbind-refused"); return S_OK; }
        bound_ = 0;
        eventLine("FIXTURE unbound");
        result->lVal = 1;
        return S_OK;
    }
    HRESULT cmpColor(DISPPARAMS* args, VARIANT* result) {
        result->vt = VT_I4;
        result->lVal = 1;
        VARIANT* x = argOf(args, 0);
        VARIANT* y = argOf(args, 1);
        VARIANT* color = argOf(args, 2);
        VARIANT* sim = argOf(args, 3);
        if (args->cArgs != 4 || !isInteger(x) || !isInteger(y) || !isText(color) || !isReal(sim)) {
            lastError_ = -44;
            eventLine("FIXTURE bad-args CmpColor");
            return S_OK;
        }
        if (!bound_) { lastError_ = -18; eventLine("FIXTURE action-unbound"); result->lVal = -1; return S_OK; }
        if (mode() == L"cmp-mismatch") return S_OK; // 业务“不匹配”，不是错误。
        COLORREF expected = 0;
        if (!parseColor(textOf(color), &expected)) return S_OK;
        result->lVal = sampleMatches(target(), x->lVal, y->lVal, expected, sim->dblVal) ? 0 : 1;
        return S_OK;
    }
    HRESULT moveTo(DISPPARAMS* args, VARIANT* result) {
        result->vt = VT_I4;
        result->lVal = 0;
        VARIANT* x = argOf(args, 0);
        VARIANT* y = argOf(args, 1);
        if (args->cArgs != 2 || !isInteger(x) || !isInteger(y)) {
            lastError_ = -44;
            eventLine("FIXTURE bad-args MoveTo");
            return S_OK;
        }
        if (!bound_) { lastError_ = -18; eventLine("FIXTURE action-unbound"); return S_OK; }
        if (mode() == L"action-fail") { lastError_ = -18; eventLine("FIXTURE action-refused"); return S_OK; }
        lastX_ = x->lVal;
        lastY_ = y->lVal;
        PostMessageW(target(), WM_MOUSEMOVE, 0,
                     MAKELPARAM(static_cast<WORD>(lastX_), static_cast<WORD>(lastY_)));
        result->lVal = 1;
        return S_OK;
    }
    HRESULT leftClick(DISPPARAMS* args, VARIANT* result) {
        result->vt = VT_I4;
        result->lVal = 0;
        if (args->cArgs != 0) return DISP_E_BADPARAMCOUNT;
        if (!bound_) { lastError_ = -18; eventLine("FIXTURE action-unbound"); return S_OK; }
        if (mode() == L"action-fail") { lastError_ = -18; eventLine("FIXTURE action-refused"); return S_OK; }
        const LPARAM point = MAKELPARAM(static_cast<WORD>(lastX_), static_cast<WORD>(lastY_));
        PostMessageW(target(), WM_LBUTTONDOWN, MK_LBUTTON, point);
        PostMessageW(target(), WM_LBUTTONUP, 0, point);
        result->lVal = 1;
        return S_OK;
    }
    HRESULT keyPress(DISPPARAMS* args, VARIANT* result) {
        result->vt = VT_I4;
        result->lVal = 0;
        VARIANT* vk = argOf(args, 0);
        if (args->cArgs != 1 || !isInteger(vk)) {
            lastError_ = -44;
            eventLine("FIXTURE bad-args KeyPress");
            return S_OK;
        }
        if (!bound_) { lastError_ = -18; eventLine("FIXTURE action-unbound"); return S_OK; }
        if (mode() == L"action-fail") { lastError_ = -18; eventLine("FIXTURE action-refused"); return S_OK; }
        PostMessageW(target(), WM_KEYDOWN, static_cast<WPARAM>(vk->lVal), 0);
        PostMessageW(target(), WM_KEYUP, static_cast<WPARAM>(vk->lVal), 0);
        result->lVal = 1;
        return S_OK;
    }
    HRESULT getLastError(DISPPARAMS* args, VARIANT* result) {
        if (args->cArgs != 0) return DISP_E_BADPARAMCOUNT;
        result->vt = VT_I4;
        result->lVal = lastError_;
        return S_OK;
    }
    HWND target() const { return reinterpret_cast<HWND>(static_cast<LONG_PTR>(bound_)); }

    std::atomic<ULONG> refs_{1};
    DWORD thread_;
    bool licensed_ = false;
    long bound_ = 0;
    long lastError_ = 0;
    long lastX_ = 0;
    long lastY_ = 0;
};

class Factory final : public IClassFactory {
public:
    Factory() { ++factories; }
    HRESULT STDMETHODCALLTYPE QueryInterface(REFIID iid, void** out) override {
        if (!out) return E_POINTER;
        *out = nullptr;
        if (iid != IID_IUnknown && iid != IID_IClassFactory) return E_NOINTERFACE;
        *out = static_cast<IClassFactory*>(this); AddRef(); return S_OK;
    }
    ULONG STDMETHODCALLTYPE AddRef() override { return ++refs_; }
    ULONG STDMETHODCALLTYPE Release() override {
        const auto left = --refs_;
        if (!left) { --factories; delete this; }
        return left;
    }
    HRESULT STDMETHODCALLTYPE CreateInstance(IUnknown* outer, REFIID iid, void** out) override {
        if (outer) return CLASS_E_NOAGGREGATION;
        auto object = new Object();
        const HRESULT hr = object->QueryInterface(iid, out);
        object->Release();
        return hr;
    }
    HRESULT STDMETHODCALLTYPE LockServer(BOOL) override { return E_NOTIMPL; }
private:
    std::atomic<ULONG> refs_{1};
};
} // namespace

extern "C" HRESULT STDAPICALLTYPE DllGetClassObject(REFCLSID clsid, REFIID iid, void** out) {
    if (clsid != fixtureId) return CLASS_E_CLASSNOTAVAILABLE;
    auto factory = new Factory();
    const HRESULT hr = factory->QueryInterface(iid, out);
    factory->Release();
    return hr;
}

extern "C" HRESULT STDAPICALLTYPE DllCanUnloadNow() {
    if (objects != 0 || factories != 0) { event("FIXTURE leaked\n"); return S_FALSE; }
    event("FIXTURE unload-ready\n");
    return S_OK;
}
