#pragma once

#include <windows.h>
#include <oaidl.h>
#include <filesystem>
#include <string>

namespace dm {

struct SdkConfig {
    std::filesystem::path dll;
    std::string sha256;
    std::wstring version;
    std::wstring clsid;
};

// 每个执行线程独占一个 Session；不得跨线程移动或传递 COM 接口。
// 绑定组合固定为文档收录的后台组合：display "gdi"、mouse "windows"、keypad "windows"、mode 0。
// 绑定前必须成功 Reg；Reg 检查自身返回码，其它业务失败读取 GetLastError。
// 每次输入/取色前核对仍绑定登记的 exact HWND。
class Session {
public:
    explicit Session(const SdkConfig& config);
    ~Session();
    Session(const Session&) = delete;
    Session& operator=(const Session&) = delete;
    std::wstring version() const;
    void registerLicense(const std::wstring& code, const std::wstring& info);

    void bindWindow(HWND hwnd);   // BindWindow(gdi,windows,windows,0)，成功后核对 GetBindWindow
    long boundWindow() const;     // GetBindWindow；0 表示未绑定
    bool cmpColor(int x, int y, const std::wstring& color, double sim); // true=匹配（业务 0）
    void moveTo(int x, int y);
    void leftClick();
    void keyPress(int vkCode);
    void unbindWindow();          // 显式解绑并核对；未绑定时为无操作

private:
    void close() noexcept;
    DISPID dispidOf(const wchar_t* name) const;
    void invoke(DISPID id, const char* operation, DISPPARAMS* arguments, VARIANT* result) const;
    long invokeLong(const wchar_t* name, const VARIANT* arguments, size_t count) const;
    void requireExactBinding(const char* operation) const;
    [[noreturn]] void fail(const char* operation) const;

    HANDLE file_ = INVALID_HANDLE_VALUE;
    HMODULE module_ = nullptr;
    IDispatch* object_ = nullptr;
    bool com_ = false;
    bool licensed_ = false;
    bool bound_ = false;
    long boundHwnd_ = 0;
};

} // namespace dm
