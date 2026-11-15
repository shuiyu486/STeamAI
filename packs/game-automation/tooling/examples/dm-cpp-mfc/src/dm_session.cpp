#include "dm_session.h"

#include <bcrypt.h>
#include <algorithm>
#include <array>
#include <cctype>
#include <iomanip>
#include <sstream>
#include <stdexcept>
#include <vector>

namespace dm {
namespace {

void require(HRESULT hr, const char* operation) {
    if (FAILED(hr)) {
        std::ostringstream text;
        text << operation << " HRESULT=0x" << std::hex << static_cast<unsigned long>(hr);
        throw std::runtime_error(text.str());
    }
}

// 仅用于错误消息；COM 方法名为 ASCII。
std::string narrow(const wchar_t* text) {
    std::string result;
    for (; *text; ++text) result.push_back(*text < 128 ? static_cast<char>(*text) : '?');
    return result;
}

// 拥有 VARIANT 所有权的小型参数包装；析构时释放 BSTR 等资源。
class Arg {
public:
    Arg(LONG integer) {
        VariantInit(&value_);
        value_.vt = VT_I4;
        value_.lVal = integer;
    }
    Arg(double real) {
        VariantInit(&value_);
        value_.vt = VT_R8;
        value_.dblVal = real;
    }
    Arg(const std::wstring& text) {
        VariantInit(&value_);
        value_.vt = VT_BSTR;
        value_.bstrVal = SysAllocStringLen(text.c_str(), static_cast<UINT>(text.size()));
        if (!value_.bstrVal) throw std::bad_alloc();
    }
    ~Arg() {
        if (value_.vt == VT_BSTR && value_.bstrVal)
            SecureZeroMemory(value_.bstrVal, SysStringByteLen(value_.bstrVal));
        VariantClear(&value_);
    }
    Arg(const Arg&) = delete;
    Arg& operator=(const Arg&) = delete;
    const VARIANT& get() const { return value_; }

private:
    VARIANT value_{};
};

std::string sha256(HANDLE file) {
    BCRYPT_ALG_HANDLE algorithm = nullptr;
    BCRYPT_HASH_HANDLE hash = nullptr;
    auto check = [](NTSTATUS status) {
        if (status < 0) throw std::runtime_error("BCrypt SHA-256 failed");
    };
    try {
        check(BCryptOpenAlgorithmProvider(&algorithm, BCRYPT_SHA256_ALGORITHM, nullptr, 0));
        check(BCryptCreateHash(algorithm, &hash, nullptr, 0, nullptr, 0, 0));
        std::array<unsigned char, 65536> buffer{};
        DWORD bytes = 0;
        for (;;) {
            if (!ReadFile(file, buffer.data(), static_cast<DWORD>(buffer.size()), &bytes, nullptr))
                throw std::runtime_error("Cannot read SDK file");
            if (bytes == 0) break;
            check(BCryptHashData(hash, buffer.data(), bytes, 0));
        }
        std::array<unsigned char, 32> digest{};
        check(BCryptFinishHash(hash, digest.data(), static_cast<ULONG>(digest.size()), 0));
        BCryptDestroyHash(hash);
        hash = nullptr;
        BCryptCloseAlgorithmProvider(algorithm, 0);
        algorithm = nullptr;
        std::ostringstream text;
        for (auto byte : digest) text << std::hex << std::setw(2) << std::setfill('0') << unsigned(byte);
        return text.str();
    } catch (...) {
        if (hash) BCryptDestroyHash(hash);
        if (algorithm) BCryptCloseAlgorithmProvider(algorithm, 0);
        throw;
    }
}

void readAt(HANDLE file, LONGLONG offset, void* output, DWORD size) {
    LARGE_INTEGER position{};
    position.QuadPart = offset;
    DWORD count = 0;
    if (!SetFilePointerEx(file, position, nullptr, FILE_BEGIN) ||
        !ReadFile(file, output, size, &count, nullptr) || count != size)
        throw std::runtime_error("Invalid SDK PE header");
}

void checkMachine(HANDLE file) {
    IMAGE_DOS_HEADER dos{};
    readAt(file, 0, &dos, sizeof(dos));
    if (dos.e_magic != IMAGE_DOS_SIGNATURE || dos.e_lfanew < sizeof(dos))
        throw std::runtime_error("Invalid SDK DOS header");
    DWORD signature = 0;
    IMAGE_FILE_HEADER pe{};
    readAt(file, dos.e_lfanew, &signature, sizeof(signature));
    readAt(file, static_cast<LONGLONG>(dos.e_lfanew) + sizeof(signature), &pe, sizeof(pe));
#ifdef _WIN64
    constexpr WORD machine = IMAGE_FILE_MACHINE_AMD64;
#else
    constexpr WORD machine = IMAGE_FILE_MACHINE_I386;
#endif
    if (signature != IMAGE_NT_SIGNATURE || !(pe.Characteristics & IMAGE_FILE_DLL))
        throw std::runtime_error("SDK file is not a PE DLL");
    if (pe.Machine != machine)
        throw std::runtime_error("SDK/worker architecture mismatch; use the matching worker");
}

} // namespace

Session::Session(const SdkConfig& config) {
    try {
        if (!config.dll.is_absolute() || config.version.empty() || config.clsid.empty() ||
            config.sha256.size() != 64 || !std::all_of(config.sha256.begin(), config.sha256.end(),
                [](unsigned char c) { return std::isxdigit(c) != 0; }))
            throw std::runtime_error("SDK requires absolute path, SHA-256, version and CLSID");
        // 禁止写入和替换本次 DLL，保持核对与加载指向同一文件。依赖目录仍须可信。
        file_ = CreateFileW(config.dll.c_str(), GENERIC_READ, FILE_SHARE_READ, nullptr,
                            OPEN_EXISTING, FILE_ATTRIBUTE_NORMAL, nullptr);
        if (file_ == INVALID_HANDLE_VALUE) throw std::runtime_error("Cannot open SDK file read-only");
        std::string expected = config.sha256;
        std::transform(expected.begin(), expected.end(), expected.begin(),
                       [](unsigned char c) { return static_cast<char>(std::tolower(c)); });
        if (sha256(file_) != expected) throw std::runtime_error("SDK SHA-256 mismatch; not loaded");
        checkMachine(file_);
        CLSID clsid{};
        require(CLSIDFromString(config.clsid.c_str(), &clsid), "CLSIDFromString");
        require(CoInitializeEx(nullptr, COINIT_MULTITHREADED), "CoInitializeEx");
        com_ = true;
        module_ = LoadLibraryExW(config.dll.c_str(), nullptr,
                                LOAD_LIBRARY_SEARCH_DLL_LOAD_DIR | LOAD_LIBRARY_SEARCH_SYSTEM32);
        if (!module_) require(HRESULT_FROM_WIN32(GetLastError()), "LoadLibraryExW");
        using FactoryFn = HRESULT(STDAPICALLTYPE*)(REFCLSID, REFIID, void**);
        auto getFactory = reinterpret_cast<FactoryFn>(GetProcAddress(module_, "DllGetClassObject"));
        if (!getFactory) throw std::runtime_error("SDK has no standard DllGetClassObject export");
        IClassFactory* factory = nullptr;
        require(getFactory(clsid, IID_IClassFactory, reinterpret_cast<void**>(&factory)), "DllGetClassObject");
        if (!factory) throw std::runtime_error("SDK returned an empty class factory");
        const HRESULT created = factory->CreateInstance(nullptr, IID_IDispatch,
                                                       reinterpret_cast<void**>(&object_));
        factory->Release();
        require(created, "CreateInstance");
        if (!object_) throw std::runtime_error("SDK returned an empty IDispatch");
        if (version() != config.version) throw std::runtime_error("SDK Ver mismatch; session stopped");
    } catch (...) {
        close();
        throw;
    }
}

DISPID Session::dispidOf(const wchar_t* name) const {
    LPOLESTR mutableName = const_cast<LPOLESTR>(name);
    DISPID id = DISPID_UNKNOWN;
    require(object_->GetIDsOfNames(IID_NULL, &mutableName, 1, LOCALE_USER_DEFAULT, &id),
            ("GetIDsOfNames(" + narrow(name) + ")").c_str());
    return id;
}

void Session::invoke(DISPID id, const char* operation, DISPPARAMS* arguments, VARIANT* result) const {
    VariantInit(result);
    EXCEPINFO exception{};
    UINT badArgument = 0;
    const HRESULT hr = object_->Invoke(id, IID_NULL, LOCALE_USER_DEFAULT, DISPATCH_METHOD,
                                       arguments, result, &exception, &badArgument);
    // EXCEPINFO 的文本可能含第三方数据；不原样写日志。
    SysFreeString(exception.bstrSource);
    SysFreeString(exception.bstrDescription);
    SysFreeString(exception.bstrHelpFile);
    require(hr, operation);
}

long Session::invokeLong(const wchar_t* name, const VARIANT* arguments, size_t count) const {
    const std::string operation = "Invoke(" + narrow(name) + ")";
    const DISPID id = dispidOf(name);
    // DISPPARAMS 按反序传参；这里只保存浅拷贝，VARIANT 所有权仍在调用方的 Arg。
    std::vector<VARIANT> reversed;
    if (count != 0) {
        reversed.assign(arguments, arguments + count);
        std::reverse(reversed.begin(), reversed.end());
    }
    DISPPARAMS params{};
    params.cArgs = static_cast<UINT>(count);
    params.rgvarg = count != 0 ? reversed.data() : nullptr;
    VARIANT result;
    invoke(id, operation.c_str(), &params, &result);
    if (result.vt != VT_I4) {
        VariantClear(&result);
        throw std::runtime_error(operation + " did not return VT_I4");
    }
    const long value = result.lVal;
    VariantClear(&result);
    return value;
}

void Session::requireExactBinding(const char* operation) const {
    if (!bound_) {
        std::ostringstream text;
        text << operation << " rejected; window not bound";
        throw std::runtime_error(text.str());
    }
    const long current = boundWindow();
    if (current != boundHwnd_) {
        std::ostringstream text;
        text << operation << " rejected; binding is " << current << ", expected " << boundHwnd_;
        throw std::runtime_error(text.str());
    }
}

void Session::fail(const char* operation) const {
    long error = 0;
    bool available = false;
    try {
        // 文档要求 GetLastError 紧跟失败调用；此处之前不得插入其它 SDK 调用。
        error = invokeLong(L"GetLastError", nullptr, 0);
        available = true;
    } catch (...) {
        // 读取失败不能伪装成大漠报告 0（无错误）。
    }
    std::ostringstream text;
    text << operation << " business-failed; GetLastError=";
    if (!available) text << "unavailable";
    else {
        text << error;
        switch (error) {
        case 0: text << " (插件未提供进一步错误原因)"; break;
        case -1: text << " (使用了收费绑定功能但未许可注册)"; break;
        case -18: text << " (窗口句柄无效)"; break;
        case -42: text << " (创建映射内存失败；检查重复绑定或句柄泄漏)"; break;
        case -43: text << " (映射内存失败；检查内存不足或泄漏)"; break;
        case -44: text << " (绑定参数无效或不受支持)"; break;
        case -45: text << " (创建互斥信号失败；检查句柄泄漏)"; break;
        default: text << " (未收录的错误码，停止并查对应版本文档)"; break;
        }
    }
    throw std::runtime_error(text.str());
}

void Session::registerLicense(const std::wstring& code, const std::wstring& info) {
    if (licensed_) throw std::runtime_error("License already registered for this session");
    if (code.empty()) throw std::runtime_error("License code is required before binding");
    Arg codeArg(code);
    Arg infoArg(info);
    const VARIANT arguments[] = {codeArg.get(), infoArg.get()};
    const long result = invokeLong(L"Reg", arguments, 2);
    if (result != 1) {
        std::ostringstream text;
        text << "Reg failed; result=" << result;
        switch (result) {
        case -1: text << " (无法连接许可服务)"; break;
        case 0: text << " (未知许可错误)"; break;
        case 2: text << " (余额不足)"; break;
        case 3: text << " (绑定机器的账户余额不足50元)"; break;
        case 4: text << " (注册码错误)"; break;
        case 5: text << " (机器或IP不符合账户许可名单)"; break;
        case 6: text << " (定制插件账户或系统语言不符合许可要求)"; break;
        case 8: text << " (附加信息不在许可白名单)"; break;
        case -8: text << " (附加信息超过40字符)"; break;
        case -9: text << " (附加信息包含非法字符)"; break;
        default: text << " (未知返回码，停止并查对应版本文档)"; break;
        }
        throw std::runtime_error(text.str());
    }
    licensed_ = true;
}

void Session::bindWindow(HWND hwnd) {
    if (!licensed_) throw std::runtime_error("BindWindow requires successful Reg first");
    if (bound_) throw std::runtime_error("BindWindow rejected; session already bound");
    if (hwnd == nullptr || !IsWindow(hwnd))
        throw std::runtime_error("BindWindow rejected; target is not a live window");
    // USER 句柄保证可放入 32 位；文档原型为整形数。
    const long handle = static_cast<long>(reinterpret_cast<LONG_PTR>(hwnd));
    Arg handleArg(handle);
    Arg displayArg(L"gdi");
    Arg mouseArg(L"windows");
    Arg keypadArg(L"windows");
    Arg modeArg(0L);
    const VARIANT arguments[] = {handleArg.get(), displayArg.get(), mouseArg.get(),
                                 keypadArg.get(), modeArg.get()};
    if (invokeLong(L"BindWindow", arguments, 5) != 1) fail("BindWindow");
    // 绑定已在 SDK 内生效：先登记再核对；核对失败或 GetBindWindow 抛出时，
    // 析构与显式解绑仍能覆盖这个已生效的绑定，不会漏掉 UnBindWindow。
    bound_ = true;
    boundHwnd_ = handle;
    const long verified = boundWindow();
    if (verified != handle)
        throw std::runtime_error("BindWindow verification failed; GetBindWindow mismatch");
}

long Session::boundWindow() const {
    return invokeLong(L"GetBindWindow", nullptr, 0);
}

bool Session::cmpColor(int x, int y, const std::wstring& color, double sim) {
    requireExactBinding("CmpColor");
    if (sim < 0.1 || sim > 1.0)
        throw std::runtime_error("CmpColor rejected; sim outside documented range 0.1-1.0");
    Arg xArg(static_cast<LONG>(x));
    Arg yArg(static_cast<LONG>(y));
    Arg colorArg(color);
    Arg simArg(sim);
    const VARIANT arguments[] = {xArg.get(), yArg.get(), colorArg.get(), simArg.get()};
    const long value = invokeLong(L"CmpColor", arguments, 4);
    if (value != 0 && value != 1)
        throw std::runtime_error("CmpColor returned undocumented value");
    return value == 0; // 文档：0=颜色匹配，1=不匹配。
}

void Session::moveTo(int x, int y) {
    requireExactBinding("MoveTo");
    Arg xArg(static_cast<LONG>(x));
    Arg yArg(static_cast<LONG>(y));
    const VARIANT arguments[] = {xArg.get(), yArg.get()};
    if (invokeLong(L"MoveTo", arguments, 2) != 1) fail("MoveTo");
}

void Session::leftClick() {
    requireExactBinding("LeftClick");
    if (invokeLong(L"LeftClick", nullptr, 0) != 1) fail("LeftClick");
}

void Session::keyPress(int vkCode) {
    requireExactBinding("KeyPress");
    Arg vkArg(static_cast<LONG>(vkCode));
    const VARIANT arguments[] = {vkArg.get()};
    if (invokeLong(L"KeyPress", arguments, 1) != 1) fail("KeyPress");
}

void Session::unbindWindow() {
    if (!bound_) return;
    if (invokeLong(L"UnBindWindow", nullptr, 0) != 1) fail("UnBindWindow");
    if (boundWindow() != 0) {
        // 仍处于绑定状态时保持 bound_，让析构兜底再尝试解绑。
        throw std::runtime_error("UnBindWindow verification failed; still bound");
    }
    bound_ = false;
    boundHwnd_ = 0;
}

std::wstring Session::version() const {
    const DISPID id = dispidOf(L"Ver");
    DISPPARAMS arguments{};
    VARIANT result;
    invoke(id, "Invoke(Ver)", &arguments, &result);
    if (result.vt != VT_BSTR || result.bstrVal == nullptr) {
        VariantClear(&result);
        throw std::runtime_error("Ver did not return BSTR");
    }
    try {
        std::wstring text(result.bstrVal, SysStringLen(result.bstrVal));
        VariantClear(&result);
        return text;
    } catch (...) {
        VariantClear(&result);
        throw;
    }
}

Session::~Session() { close(); }

void Session::close() noexcept {
    if (object_ && bound_) {
        // 结束路径兜底解绑，尽力而为且不抛出；显式成功证据必须来自 unbindWindow()。
        try { (void)invokeLong(L"UnBindWindow", nullptr, 0); } catch (...) {}
    }
    bound_ = false;
    boundHwnd_ = 0;
    if (object_) { object_->Release(); object_ = nullptr; }
    if (com_) { CoUninitialize(); com_ = false; }
    if (module_) {
        using UnloadFn = HRESULT(STDAPICALLTYPE*)();
        auto canUnload = reinterpret_cast<UnloadFn>(GetProcAddress(module_, "DllCanUnloadNow"));
        // DLL 未确认可卸载时留至本次短生命周期进程退出，绝不强行卸载。
        if (canUnload && canUnload() == S_OK) FreeLibrary(module_);
        module_ = nullptr;
    }
    if (file_ != INVALID_HANDLE_VALUE) { CloseHandle(file_); file_ = INVALID_HANDLE_VALUE; }
}

} // namespace dm
