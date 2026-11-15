#include "mfc_window.h"

#include "license_input.h"
#include "mfc_selftest.h"

#include <afxdlgs.h>
#include <bcrypt.h>

#include <cctype>
#include <stdexcept>
#include <vector>

namespace dm {

namespace {

bool isSha256Text(const CString& text) {
    if (text.GetLength() != 64) return false;
    for (int i = 0; i < text.GetLength(); ++i) {
        const wchar_t c = text[i];
        if (!iswxdigit(c)) return false;
    }
    return true;
}

bool isClsidText(const CString& text) {
    // {8-4-4-4-12}
    static const int groups[] = {8, 4, 4, 4, 12};
    if (text.GetLength() != 38 || text[0] != L'{' || text[37] != L'}') return false;
    int index = 1;
    for (int group : groups) {
        for (int i = 0; i < group; ++i, ++index)
            if (!iswxdigit(text[index])) return false;
        if (index < 37 && text[index] != L'-') return false;
        ++index;
    }
    return true;
}

// 本次许可文案必须与实际动作一致：只查版本 / Reg 注册＋绑定自建测试窗口两档。
const wchar_t* const kApproveVerLabel =
    L"允许本次加载上面指定的 DLL 并查询 Ver（不注册、不调用付费许可、不操作窗口）";
const wchar_t* const kApproveWindowLabel =
    L"允许本次加载上面指定的 DLL 并完成 Reg 许可注册（可能联网/计费），随后绑定我们自建目标窗口取色、点击、按键，不碰其它窗口";

} // namespace

bool Sha256File(const std::wstring& path, std::wstring& digest) {
    digest.clear();
    HANDLE file = CreateFileW(path.c_str(), GENERIC_READ, FILE_SHARE_READ, nullptr,
                              OPEN_EXISTING, FILE_ATTRIBUTE_NORMAL, nullptr);
    if (file == INVALID_HANDLE_VALUE) return false;
    BCRYPT_ALG_HANDLE algorithm = nullptr;
    BCRYPT_HASH_HANDLE hash = nullptr;
    bool ok = false;
    std::wstring result;
    if (BCryptOpenAlgorithmProvider(&algorithm, BCRYPT_SHA256_ALGORITHM, nullptr, 0) >= 0 &&
        BCryptCreateHash(algorithm, &hash, nullptr, 0, nullptr, 0, 0) >= 0) {
        std::vector<unsigned char> buffer(65536);
        bool readFailed = false;
        for (;;) {
            DWORD read = 0;
            if (!ReadFile(file, buffer.data(), static_cast<DWORD>(buffer.size()), &read, nullptr)) {
                readFailed = true;
                break;
            }
            if (read == 0) break;
            if (BCryptHashData(hash, buffer.data(), read, 0) < 0) { readFailed = true; break; }
        }
        if (!readFailed) {
            unsigned char bytes[32];
            if (BCryptFinishHash(hash, bytes, sizeof(bytes), 0) >= 0) {
                wchar_t hex[65];
                for (int i = 0; i < 32; ++i)
                    swprintf_s(&hex[i * 2], 3, L"%02x", bytes[i]);
                hex[64] = L'\0';
                result = hex;
                ok = true;
            }
        }
    }
    if (hash) BCryptDestroyHash(hash);
    if (algorithm) BCryptCloseAlgorithmProvider(algorithm, 0);
    CloseHandle(file);
    if (ok) digest = result;
    return ok;
}

MainWindow::MainWindow() = default;

MainWindow::~MainWindow() {
    // 兜底：窗口销毁而 worker 仍在时，先协作请求停止并等它（及其目标）结束，再关桌面。
    // 不 SwitchDesktop、不 SendInput、不强杀；阻塞调用返回不了就如实超时放弃等待。
    if (testDesktop_) {
        if (worker_.running()) {
            worker_.writeLine("stop");
            const DWORD started = GetTickCount();
            while (!worker_.exited() && GetTickCount() - started < 5000) {
                worker_.poll();
                Sleep(10);
            }
        }
        worker_.reset();  // 关闭私有管道（EOF）与全部句柄；目标随 worker 链式退出
        CloseDesktop(testDesktop_);
        testDesktop_ = nullptr;
        testDesktopName_.clear();
    }
}

void MainWindow::createTestDesktop() {
    // 唯一非活动桌面：worker 与自建目标在其上默认 show 并正常绘制/取色，用户桌面不受影响
    const std::wstring name = L"dm_mfc_" + std::to_wstring(GetCurrentProcessId()) + L"_" +
                              std::to_wstring(GetTickCount());
    testDesktop_ = CreateDesktopW(name.c_str(), nullptr, nullptr, 0, GENERIC_ALL, nullptr);
    if (testDesktop_ != nullptr) testDesktopName_ = name;
}

void MainWindow::closeTestDesktop() {
    if (testDesktop_ == nullptr) return;
    CloseDesktop(testDesktop_);
    testDesktop_ = nullptr;
    testDesktopName_.clear();
}

BEGIN_MESSAGE_MAP(MainWindow, CFrameWnd)
    ON_WM_CREATE()
    ON_WM_SIZE()
    ON_WM_TIMER()
    ON_WM_CLOSE()
    ON_WM_NCDESTROY()
    ON_EN_CHANGE(IDC_WORKER_PATH, OnFieldChanged)
    ON_EN_CHANGE(IDC_DLL_PATH, OnFieldChanged)
    ON_EN_CHANGE(IDC_SHA256, OnFieldChanged)
    ON_EN_CHANGE(IDC_SDK_VERSION, OnFieldChanged)
    ON_EN_CHANGE(IDC_CLSID, OnFieldChanged)
    ON_EN_CHANGE(IDC_REG_CODE, OnFieldChanged)
    ON_EN_CHANGE(IDC_VER_INFO, OnFieldChanged)
    ON_BN_CLICKED(IDC_DEMO, OnDemoToggled)
    ON_BN_CLICKED(IDC_WINDOW_DEMO, OnWindowDemoToggled)
    ON_BN_CLICKED(IDC_DLL_BROWSE, OnBrowseClicked)
    ON_BN_CLICKED(IDC_SHA_COMPUTE, OnHashClicked)
    ON_BN_CLICKED(IDC_START, OnStartClicked)
    ON_BN_CLICKED(IDC_PAUSE, OnPauseClicked)
    ON_BN_CLICKED(IDC_STOP, OnStopClicked)
END_MESSAGE_MAP()

int MainWindow::OnCreate(LPCREATESTRUCT createStruct) {
    if (CFrameWnd::OnCreate(createStruct) != 0) return -1;
    font_.CreateFontW(-12, 0, 0, 0, FW_NORMAL, 0, 0, 0, DEFAULT_CHARSET, OUT_DEFAULT_PRECIS,
                      CLIP_DEFAULT_PRECIS, CLEARTYPE_QUALITY, DEFAULT_PITCH | FF_DONTCARE,
                      L"Microsoft YaHei UI");
    const DWORD labelStyle = WS_CHILD | WS_VISIBLE | SS_RIGHT;
    const DWORD editStyle = WS_CHILD | WS_VISIBLE | WS_TABSTOP | WS_BORDER | ES_AUTOHSCROLL;
    const DWORD buttonStyle = WS_CHILD | WS_VISIBLE | WS_TABSTOP;
    const DWORD checkStyle = WS_CHILD | WS_VISIBLE | WS_TABSTOP | BS_AUTOCHECKBOX;
    CRect empty(0, 0, 0, 0);
    struct Row {
        CStatic* label;
        const wchar_t* text;
        CEdit* edit;
    } rows[] = {
        {&workerLabel_, L"执行程序", &workerEdit_},
        {&dllLabel_, L"大漠 DLL", &dllEdit_},
        {&shaLabel_, L"文件校验", &shaEdit_},
        {&versionLabel_, L"预期版本", &versionEdit_},
        {&clsidLabel_, L"COM 类标识", &clsidEdit_},
    };
    const int rowIds[] = {IDC_WORKER_PATH, IDC_DLL_PATH, IDC_SHA256, IDC_SDK_VERSION, IDC_CLSID};
    for (int i = 0; i < 5; ++i) {
        rows[i].label->Create(rows[i].text, labelStyle, empty, this, 1201 + i);  // 仅布局用
        rows[i].edit->Create(editStyle, empty, this, rowIds[i]);
        rows[i].label->SetFont(&font_);
        rows[i].edit->SetFont(&font_);
    }
    regLabel_.Create(L"注册码", labelStyle, empty, this, 1206);
    regEdit_.Create(editStyle | ES_PASSWORD, empty, this, IDC_REG_CODE);
    verInfoLabel_.Create(L"注册 ver_info", labelStyle, empty, this, 1207);
    verInfoEdit_.Create(editStyle, empty, this, IDC_VER_INFO);
    regLabel_.SetFont(&font_);
    regEdit_.SetFont(&font_);
    verInfoLabel_.SetFont(&font_);
    verInfoEdit_.SetFont(&font_);
    regEdit_.SetLimitText(512);
    verInfoEdit_.SetLimitText(40);
    browseButton_.Create(L"选择 DLL", buttonStyle, empty, this, IDC_DLL_BROWSE);
    hashButton_.Create(L"只读计算 SHA-256", buttonStyle, empty, this, IDC_SHA_COMPUTE);
    demoCheck_.Create(L"演示模式：不加载大漠，只测试开始、暂停和停止", checkStyle, empty, this, IDC_DEMO);
    approveCheck_.Create(kApproveVerLabel, checkStyle | BS_MULTILINE, empty, this, IDC_APPROVE);
    windowDemoCheck_.Create(L"绑定自建测试窗口（取色、点击、按键），不碰其它窗口", checkStyle, empty,
                            this, IDC_WINDOW_DEMO);
    startButton_.Create(L"开始", buttonStyle, empty, this, IDC_START);
    pauseButton_.Create(L"暂停", buttonStyle, empty, this, IDC_PAUSE);
    stopButton_.Create(L"停止", buttonStyle, empty, this, IDC_STOP);
    stateLabel_.Create(L"未运行", WS_CHILD | WS_VISIBLE | SS_LEFT | SS_CENTERIMAGE, empty, this, IDC_STATE);
    noteLabel_.Create(L"此版本不含游戏流程。暂停/停止是协作式请求；正在执行的 DLL 调用需返回后才能释放。",
                      WS_CHILD | WS_VISIBLE | SS_LEFT | SS_EDITCONTROL, empty, this, static_cast<UINT>(-1));
    logBox_.Create(WS_CHILD | WS_VISIBLE | WS_BORDER | WS_VSCROLL | LBS_NOTIFY | LBS_NOINTEGRALHEIGHT,
                   empty, this, IDC_LOG);
    CWnd* const controls[] = {&browseButton_, &hashButton_, &demoCheck_, &approveCheck_,
                              &windowDemoCheck_, &startButton_,
                              &pauseButton_, &stopButton_, &stateLabel_, &noteLabel_, &logBox_};
    for (CWnd* control : controls) control->SetFont(&font_);
    wchar_t module[MAX_PATH] = {};
    GetModuleFileNameW(nullptr, module, MAX_PATH);
    if (wchar_t* slash = wcsrchr(module, L'\\')) *slash = L'\0';
    SetDlgItemText(IDC_WORKER_PATH, (std::wstring(module) + L"\\dm_worker.exe").c_str());
    SetDlgItemText(IDC_DLL_PATH, L"");
    shaEdit_.SetLimitText(64);
    demoCheck_.SetCheck(BST_CHECKED);  // 演示默认：不加载大漠
    approveCheck_.EnableWindow(FALSE);
    windowDemoCheck_.SetCheck(BST_UNCHECKED);  // 默认不勾；仅 SDK 模式可选
    windowDemoCheck_.EnableWindow(FALSE);
    regEdit_.EnableWindow(FALSE);
    verInfoEdit_.EnableWindow(FALSE);
    controlsReady_ = true;
    layout();
    idle();
    return 0;
}

void MainWindow::layout() {
    if (!controlsReady_) return;
    CRect client;
    GetClientRect(&client);
    const int cx = client.Width();
    const int margin = 12, labelW = 84;
    const int rowY[] = {10, 42, 74, 106, 138, 170, 202};
    const int rowH = 24;
    CWnd* labels[] = {&workerLabel_, &dllLabel_, &shaLabel_, &versionLabel_, &clsidLabel_,
                      &regLabel_, &verInfoLabel_};
    CWnd* edits[] = {&workerEdit_, &dllEdit_, &shaEdit_, &versionEdit_, &clsidEdit_,
                     &regEdit_, &verInfoEdit_};
    for (int i = 0; i < 7; ++i) {
        labels[i]->MoveWindow(margin, rowY[i], labelW, rowH);
        int editRight = cx - margin;
        if (i == 1) editRight -= 96 + 8;  // 选择 DLL 按钮
        if (i == 2) editRight -= 128 + 8; // 计算 SHA-256 按钮
        edits[i]->MoveWindow(margin + labelW + 8, rowY[i], editRight - (margin + labelW + 8), rowH);
    }
    browseButton_.MoveWindow(cx - margin - 96, rowY[1], 96, rowH);
    hashButton_.MoveWindow(cx - margin - 128, rowY[2], 128, rowH);
    demoCheck_.MoveWindow(margin, 234, cx - margin * 2, 20);
    approveCheck_.MoveWindow(margin, 256, cx - margin * 2, 40);
    windowDemoCheck_.MoveWindow(margin, 300, cx - margin * 2, 20);
    startButton_.MoveWindow(margin, 326, 90, 30);
    pauseButton_.MoveWindow(margin + 98, 326, 90, 30);
    stopButton_.MoveWindow(margin + 196, 326, 90, 30);
    stateLabel_.MoveWindow(margin + 294, 326, cx - margin - (margin + 294), 30);
    const int logTop = 362;
    const int logBottom = cx > 0 ? client.Height() - 44 : logTop + 100;
    if (logBottom >= logTop + 40) logBox_.MoveWindow(margin, logTop, cx - margin * 2, logBottom - logTop);
    noteLabel_.MoveWindow(margin, client.Height() - 40, cx - margin * 2, 36);
}

void MainWindow::OnSize(UINT type, int cx, int cy) {
    CFrameWnd::OnSize(type, cx, cy);
    layout();
}

void MainWindow::log(const CString& text) {
    SYSTEMTIME now{};
    GetLocalTime(&now);
    CString line;
    line.Format(L"%02u:%02u:%02u.%03u %s", now.wHour, now.wMinute, now.wSecond, now.wMilliseconds,
                static_cast<LPCTSTR>(text));
    logBox_.AddString(line);
    while (logBox_.GetCount() > 1000) logBox_.DeleteString(0);  // 有限日志
}

void MainWindow::setState(const CString& text) { SetDlgItemText(IDC_STATE, text); }

void MainWindow::idle() {
    startButton_.EnableWindow(TRUE);
    pauseButton_.EnableWindow(FALSE);
    SetDlgItemText(IDC_PAUSE, L"暂停");
    stopButton_.EnableWindow(FALSE);
    demoCheck_.EnableWindow(TRUE);
    // 绑定自建测试窗口与加载许可一样，只在 SDK 模式（非演示）下可选
    const BOOL sdkMode = demoCheck_.GetCheck() == BST_CHECKED ? FALSE : TRUE;
    approveCheck_.EnableWindow(sdkMode);
    windowDemoCheck_.EnableWindow(sdkMode);
    syncWindowDemoFields();
    setEditsReadonly(FALSE);
    stopping_ = false;
    paused_ = false;
}

void MainWindow::setEditsReadonly(BOOL readOnly) {
    for (CEdit* edit : {&workerEdit_, &dllEdit_, &shaEdit_, &versionEdit_, &clsidEdit_,
                        &regEdit_, &verInfoEdit_})
        edit->ModifyStyle(readOnly ? 0 : ES_READONLY, readOnly ? ES_READONLY : 0);
}

void MainWindow::syncWindowDemoFields() {
    // 注册码/ver_info 仅在窗口测试模式下需要填写
    const BOOL windowMode = demoCheck_.GetCheck() != BST_CHECKED &&
                                    windowDemoCheck_.GetCheck() == BST_CHECKED
                                ? TRUE
                                : FALSE;
    regEdit_.EnableWindow(windowMode);
    verInfoEdit_.EnableWindow(windowMode);
}

void MainWindow::updateApproveLabel() {
    // 许可文案与实际动作保持一致：勾选绑定自建测试窗口时是绑定档，否则仅版本查询档
    SetDlgItemText(IDC_APPROVE, windowDemoCheck_.GetCheck() == BST_CHECKED ? kApproveWindowLabel
                                                                           : kApproveVerLabel);
}

std::wstring MainWindow::absoluteExistingFilePath(int id) const {
    CString text;
    const_cast<MainWindow*>(this)->GetDlgItemText(id, text);
    text.Trim();
    if (text.IsEmpty()) return {};
    wchar_t path[MAX_PATH] = {};
    const DWORD attributes = GetFileAttributesW(text.GetString());
    if (attributes == INVALID_FILE_ATTRIBUTES || (attributes & FILE_ATTRIBUTE_DIRECTORY)) return {};
    if (!GetFullPathNameW(text.GetString(), MAX_PATH, path, nullptr)) return {};
    return path;
}

void MainWindow::OnStartClicked() {
    if (worker_.running() || stopping_) return;  // 停机期间不新启动
    const std::wstring executable = absoluteExistingFilePath(IDC_WORKER_PATH);
    if (executable.empty()) {
        log(L"找不到执行程序；先编译或填写 dm_worker.exe 的位置。");
        return;
    }
    std::wstring arguments;
    dm::LicenseInput license;  // 注册码只在内存里走到私有管道，不进命令行、日志或文件
    bool sendLicense = false;
    if (demoCheck_.GetCheck() == BST_CHECKED) {
        arguments = L"--demo";
    } else {
        const std::wstring dll = absoluteExistingFilePath(IDC_DLL_PATH);
        CString sha, version, clsid;
        GetDlgItemText(IDC_SHA256, sha); sha.Trim();
        GetDlgItemText(IDC_SDK_VERSION, version); version.Trim();
        GetDlgItemText(IDC_CLSID, clsid); clsid.Trim();
        const bool windowMode = windowDemoCheck_.GetCheck() == BST_CHECKED;
        if (windowMode) {
            CString code, info;
            GetDlgItemText(IDC_REG_CODE, code); code.Trim();
            GetDlgItemText(IDC_VER_INFO, info); info.Trim();
            license.code = code.GetString();
            license.info = info.GetString();
        }
        const bool approved = approveCheck_.GetCheck() == BST_CHECKED;
        const bool complete = approved && !dll.empty() && !version.IsEmpty() &&
                              isSha256Text(sha) && isClsidText(clsid) &&
                              (!windowMode || !license.code.empty());
        if (!complete) {
            log(windowMode
                    ? L"请补齐 DLL、SHA-256、预期版本、CLSID 与注册码，并明确允许本次绑定自建测试窗口。（ver_info 可空，不是 SDK 预期版本）"
                    : L"请补齐 DLL、SHA-256、预期版本、CLSID，并明确允许本次版本查询。");
            license.clear();
            return;
        }
        if (windowMode) {
            try {
                dm::validateLicenseInput(license);
            } catch (const std::exception&) {
                log(L"注册码或 ver_info 超出长度/字符限制；未启动。");
                license.clear();
                return;
            }
            sendLicense = true;
        }
        arguments = L"--sdk-approved --dll \"" + dll + L"\" --sha256 " + std::wstring(sha.GetString()) +
                    L" --version \"" + std::wstring(version.GetString()) + L"\" --clsid " +
                    std::wstring(clsid.GetString());
        if (windowMode) {
            // 绑定自建测试窗口：目标固定为 worker 同目录的 dm_demo_target.exe，由 worker
            // 启动与回收；UI 不另起 SDK 调用或新 runner。隐藏验收走隔离桌面，目标在其上
            // 默认 show 以便绘制/取色，但用户输入桌面不变，因此不传 --hidden-target。
            const std::size_t slash = executable.find_last_of(L"\\/");
            const std::wstring directory =
                slash == std::wstring::npos ? std::wstring() : executable.substr(0, slash);
            arguments += L" --window-demo --target \"" + directory + L"\\dm_demo_target.exe\"";
            arguments += L" --reg-approved --license-stdin";
        }
    }
    approveCheck_.SetCheck(BST_UNCHECKED);
    approveCheck_.EnableWindow(FALSE);
    demoCheck_.EnableWindow(FALSE);
    windowDemoCheck_.EnableWindow(FALSE);
    setEditsReadonly(TRUE);
    paused_ = false;
    closing_ = false;
    stopping_ = false;
    startButton_.EnableWindow(FALSE);
    stopButton_.EnableWindow(TRUE);
    pauseButton_.EnableWindow(FALSE);
    setState(L"启动中");
    std::wstring error;
    if (!worker_.start(executable, arguments, error, testDesktop_ ? testDesktopName_ : std::wstring())) {
        log(CString(L"执行程序错误：") + error.c_str());
        license.clear();
        closeTestDesktop();
        idle();
        setState(L"启动失败");
        if (closing_ && WindowAlive()) DestroyWindow();
        return;
    }
    SetTimer(TIMER_POLL_WORKER, 50, nullptr);
    if (sendLicense) {
        // 启动成功后经既有私有管道发送一行 UTF-8 初始化（LICENSE\t注册码\tver_info），
        // 之后立即清空密码框并擦除本地副本；注册码不出现在任何日志或命令行。
        bool sent = false;
        std::string initialization;
        try {
            initialization = dm::licenseLine(license);
            sent = worker_.writeLine(initialization);
        } catch (const std::exception&) {
            sent = false;
        }
        if (!initialization.empty())
            SecureZeroMemory(initialization.data(), initialization.size());
        SetDlgItemText(IDC_REG_CODE, L"");
        license.clear();
        if (!sent) {
            log(L"许可初始化行发送失败；已请求停止，等待执行程序退出。");
            requestStop();
            return;
        }
    } else {
        license.clear();
    }
    setState(L"执行程序已启动");
    pauseButton_.EnableWindow(TRUE);
}

void MainWindow::OnPauseClicked() {
    if (!worker_.running() || stopping_) return;
    paused_ = !paused_;
    if (!worker_.writeLine(paused_ ? "pause" : "resume")) log(L"控制命令未发送成功。");
    SetDlgItemText(IDC_PAUSE, paused_ ? L"继续" : L"暂停");
    setState(paused_ ? L"已请求暂停" : L"已请求继续");
}

void MainWindow::OnStopClicked() { requestStop(); }

void MainWindow::requestStop() {
    if (!worker_.running() || stopping_) return;  // stop 只发一次，重复点击不重复写管道
    stopping_ = true;
    if (!worker_.writeLine("stop")) log(L"控制命令未发送成功。");
    pauseButton_.EnableWindow(FALSE);
    stopButton_.EnableWindow(FALSE);
    setState(L"正在停止并等待释放");
    SetTimer(TIMER_STOP_DEADLINE, 2000, nullptr);
}

void MainWindow::OnTimer(UINT_PTR id) {
    if (id == TIMER_POLL_WORKER && worker_.active()) {
        worker_.poll();
        std::deque<std::wstring> lines;
        worker_.takeLines(lines);
        for (const auto& line : lines)
            if (!line.empty()) log(line.c_str());
        if (worker_.exited()) handleWorkerExit();
        return;
    }
    if (id == TIMER_STOP_DEADLINE) {
        KillTimer(TIMER_STOP_DEADLINE);
        if (worker_.running()) {
            setState(L"停止超过 2 秒，仍在等待");
            log(L"未满足停止时限；没有强杀线程或宣称清理成功。DLL 调用可能仍在阻塞。");
        }
        return;
    }
    if (id == TIMER_SMOKE_POLL && smoke_) smoke_->Poll();
}

void MainWindow::handleWorkerExit() {
    KillTimer(TIMER_STOP_DEADLINE);
    KillTimer(TIMER_POLL_WORKER);
    lastExitCode_ = worker_.exitCode();
    CString text;
    text.Format(L"执行程序退出：code=%d", lastExitCode_);
    log(text);
    if (smoke_) smoke_->OnWorkerExitObserved();  // 销毁窗口前，日志仍完整
    worker_.reset();
    closeTestDesktop();  // worker（及其目标）已结束，回收隔离桌面
    idle();
    setState(lastExitCode_ == 0 ? L"已停止" : L"运行失败");
    if (closing_ && WindowAlive()) DestroyWindow();
}

void MainWindow::OnFieldChanged() {
    // 显式本次许可：任一字段变化即撤销，防止拿旧许可加载新 DLL。
    approveCheck_.SetCheck(BST_UNCHECKED);
}

void MainWindow::OnDemoToggled() {
    approveCheck_.SetCheck(BST_UNCHECKED);
    const BOOL sdkMode = demoCheck_.GetCheck() == BST_CHECKED || worker_.running() ? FALSE : TRUE;
    if (demoCheck_.GetCheck() == BST_CHECKED) windowDemoCheck_.SetCheck(BST_UNCHECKED);  // 仅 SDK 模式可选
    updateApproveLabel();
    approveCheck_.EnableWindow(sdkMode);
    windowDemoCheck_.EnableWindow(sdkMode);
    syncWindowDemoFields();
}

void MainWindow::OnWindowDemoToggled() {
    // 绑定自建测试窗口是新的授权范围：切换即撤销未使用的本次加载许可，并同步许可文案与输入区
    approveCheck_.SetCheck(BST_UNCHECKED);
    updateApproveLabel();
    syncWindowDemoFields();
}

void MainWindow::OnBrowseClicked() {
    CFileDialog dialog(TRUE, L"dll", nullptr, OFN_HIDEREADONLY | OFN_FILEMUSTEXIST, L"DLL (*.dll)|*.dll||", this);
    if (dialog.DoModal() == IDOK) SetDlgItemText(IDC_DLL_PATH, dialog.GetPathName());
}

void MainWindow::OnHashClicked() {
    const std::wstring dll = absoluteExistingFilePath(IDC_DLL_PATH);
    std::wstring digest;
    if (dll.empty() || !Sha256File(dll, digest)) {
        log(L"文件读取失败；没有加载 DLL。");
        return;
    }
    SetDlgItemText(IDC_SHA256, digest.c_str());  // 触发 EN_CHANGE，撤销未用许可
}

void MainWindow::OnClose() {
    if (!worker_.running()) {
        CFrameWnd::OnClose();
        return;
    }
    closing_ = true;  // 正常退出等 worker 结束后再销毁窗口
    requestStop();
}

void MainWindow::OnNcDestroy() {
    if (smoke_) smoke_->OnWindowDestroyed();
    CFrameWnd::OnNcDestroy();
}

// ---- 自测辅助：与用户相同的控件路径 ----

CString MainWindow::GetText(int id) const {
    CString text;
    const_cast<MainWindow*>(this)->GetDlgItemText(id, text);
    return text;
}

void MainWindow::SetText(int id, const CString& text) {
    const_cast<MainWindow*>(this)->SetDlgItemText(id, text);
}

BOOL MainWindow::IsEnabled(int id) const {
    CWnd* control = const_cast<MainWindow*>(this)->GetDlgItem(id);
    return control ? control->IsWindowEnabled() : FALSE;
}

int MainWindow::CheckState(int id) const {
    return const_cast<MainWindow*>(this)->IsDlgButtonChecked(id);
}

void MainWindow::ClickControl(int id) {
    if (CWnd* control = GetDlgItem(id)) control->SendMessage(BM_CLICK, 0, 0);
}

void MainWindow::RequestClose() { SendMessage(WM_CLOSE, 0, 0); }

int MainWindow::LogCount() const { return logBox_.GetCount(); }

CString MainWindow::LogLine(int index) const {
    CString line;
    logBox_.GetText(index, line);
    return line;
}

BOOL MainWindow::LogContainsFrom(int index, LPCTSTR text) const {
    CString line;
    for (int i = index < 0 ? 0 : index; i < logBox_.GetCount(); ++i) {
        logBox_.GetText(i, line);
        if (line.Find(text) >= 0) return TRUE;
    }
    return FALSE;
}

void MainWindow::RunSmoke(const std::wstring& worker, const std::wstring& fixture,
                          const std::wstring& resultFile) {
    if (!worker.empty()) SetDlgItemText(IDC_WORKER_PATH, worker.c_str());  // 空则用默认同目录路径
    smoke_ = std::make_unique<SmokeTest>(*this, worker, fixture, resultFile);
    smoke_->Start();
}

void MainWindow::RunWindowDemoSmoke(const std::wstring& worker, const WindowDemoSpec& spec,
                                    const std::wstring& resultFile) {
    if (!worker.empty()) SetDlgItemText(IDC_WORKER_PATH, worker.c_str());
    // 隐藏验收必须先有隔离桌面：worker 与目标在其上绘制，用户桌面不出现任何窗口。
    // 创建失败即整体拒绝，绝不退回在用户桌面显示目标。
    createTestDesktop();
    if (testDesktop_ == nullptr) {
        const std::vector<std::string> lines{"FAIL window-demo-smoke rejected: cannot create isolated desktop"};
        WriteSmokeResultFile(resultFile, lines, true);
        RecordSmokeExitCode(1);
        DestroyForSmoke();
        return;
    }
    smoke_ = std::make_unique<SmokeTest>(*this, worker, resultFile, spec);
    smoke_->Start();
}

} // namespace dm
