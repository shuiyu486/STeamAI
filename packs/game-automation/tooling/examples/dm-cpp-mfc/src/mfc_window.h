#pragma once

#include <afxwin.h>  // MFC 头必须先于 windows.h

#include "mfc_worker_process.h"
#include "resource.h"

#include <memory>
#include <string>

constexpr UINT_PTR TIMER_POLL_WORKER = 1;     // 50ms：有界读取 worker 输出
constexpr UINT_PTR TIMER_STOP_DEADLINE = 2;   // 2s：停止时限的如实提示
constexpr UINT_PTR TIMER_SMOKE_POLL = 3;      // 100ms：隐藏自测推进

namespace dm {

class SmokeTest;
struct WindowDemoSpec;

// 只读计算文件 SHA-256；失败返回 false。不加载、不注册。
bool Sha256File(const std::wstring& path, std::wstring& digest);

class MainWindow : public CFrameWnd {
public:
    MainWindow();
    ~MainWindow();  // 隔离桌面兜底回收：先结束 worker/目标，再关桌面句柄

    // 自测使用与用户完全相同的控件消息路径
    BOOL IsWorkerRunning() const { return worker_.running(); }
    int LastExitCode() const { return lastExitCode_; }
    unsigned long WorkerTicks() const { return worker_.ticks(); }
    CString GetText(int id) const;
    void SetText(int id, const CString& text);
    BOOL IsEnabled(int id) const;
    int CheckState(int id) const;
    void ClickControl(int id);       // BM_CLICK：真实按钮/复选框路径
    void RequestClose();             // WM_CLOSE：与点击标题栏 X 相同
    BOOL WindowAlive() const { return ::IsWindow(GetSafeHwnd()); }
    int LogCount() const;
    CString LogLine(int index) const;
    BOOL LogContainsFrom(int index, LPCTSTR text) const;
    void DestroyForSmoke() { if (WindowAlive()) DestroyWindow(); }
    void RunSmoke(const std::wstring& worker, const std::wstring& fixture,
                  const std::wstring& resultFile);
    // --window-demo-smoke 隐藏验收入口：预填真实控件并勾选后由 SmokeTest 用 BM_CLICK 驱动
    void RunWindowDemoSmoke(const std::wstring& worker, const WindowDemoSpec& spec,
                            const std::wstring& resultFile);

protected:
    afx_msg int OnCreate(LPCREATESTRUCT createStruct);
    afx_msg void OnSize(UINT type, int cx, int cy);
    afx_msg void OnTimer(UINT_PTR id);
    afx_msg void OnClose();
    afx_msg void OnNcDestroy();
    afx_msg void OnFieldChanged();
    afx_msg void OnDemoToggled();
    afx_msg void OnWindowDemoToggled();
    afx_msg void OnBrowseClicked();
    afx_msg void OnHashClicked();
    afx_msg void OnStartClicked();
    afx_msg void OnPauseClicked();
    afx_msg void OnStopClicked();
    DECLARE_MESSAGE_MAP()

private:
    void layout();
    void log(const CString& text);
    void setState(const CString& text);
    void idle();
    void requestStop();
    void handleWorkerExit();
    void setEditsReadonly(BOOL readOnly);
    void updateApproveLabel();
    void syncWindowDemoFields();
    void createTestDesktop();   // 隐藏窗口验收：唯一非活动桌面，worker/目标在其上默认绘制
    void closeTestDesktop();
    std::wstring absoluteExistingFilePath(int id) const;

    CStatic workerLabel_, dllLabel_, shaLabel_, versionLabel_, clsidLabel_;
    CEdit workerEdit_, dllEdit_, shaEdit_, versionEdit_, clsidEdit_;
    CStatic regLabel_, verInfoLabel_;      // 注册码/ver_info：仅窗口测试需要，不持久化
    CEdit regEdit_, verInfoEdit_;
    CButton browseButton_, hashButton_;
    CButton demoCheck_, approveCheck_, windowDemoCheck_;
    CButton startButton_, pauseButton_, stopButton_;
    CStatic stateLabel_, noteLabel_;
    CListBox logBox_;
    CFont font_;

    WorkerProcess worker_;
    std::unique_ptr<SmokeTest> smoke_;
    HDESK testDesktop_ = nullptr;      // 隐藏窗口验收的隔离桌面；可见用户模式恒为空
    std::wstring testDesktopName_;
    int lastExitCode_ = -1;
    bool paused_ = false;
    bool stopping_ = false;
    bool closing_ = false;
    bool controlsReady_ = false;
};

} // namespace dm
