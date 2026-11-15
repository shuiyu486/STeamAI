#include "mfc_selftest.h"

#include "resource.h"

namespace {

constexpr wchar_t FIXTURE_CLSID[] = L"{9F174D53-4C94-4710-9904-100120023003}";
constexpr wchar_t FIXTURE_VERSION[] = L"fixture-1";
constexpr wchar_t SHA_PLACEHOLDER[] = L"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef";
const DWORD STEP_TIMEOUT = 10000;
// window-demo 观察段覆盖目标进程启动与全部绑定/取色/点击/按键序列，真实 DLL 也在用，放宽步限。
const DWORD WINDOW_DEMO_STEP_TIMEOUT = 30000;
const DWORD OVERALL_TIMEOUT = 120000;

// worker --window-demo 的终态序列；必须按此顺序出现，最后 STATE stopped + exit 0。
const wchar_t* const kWindowDemoMarkers[] = {
    L"DEMO target-ready", L"DEMO bound", L"DEMO color-ok", L"DEMO click-ok",
    L"DEMO key-ok",       L"DEMO unbound", L"DEMO target-closed", L"DEMO pass",
};
constexpr int kWindowDemoMarkerCount = 8;

bool expired(DWORD now, DWORD deadline) { return static_cast<int>(now - deadline) >= 0; }

std::string toUtf8(const CStringW& text) {
    if (text.IsEmpty()) return {};
    const int size = WideCharToMultiByte(CP_UTF8, 0, text.GetString(), text.GetLength(), nullptr, 0,
                                         nullptr, nullptr);
    std::string utf8(size, '\0');
    WideCharToMultiByte(CP_UTF8, 0, text.GetString(), text.GetLength(), utf8.data(), size, nullptr,
                        nullptr);
    return utf8;
}

} // namespace

namespace dm {

SmokeTest::SmokeTest(MainWindow& window, std::wstring worker, std::wstring fixture,
                     std::wstring resultFile)
    : window_(window), workerPath_(std::move(worker)), fixture_(std::move(fixture)),
      resultFile_(std::move(resultFile)) {}

SmokeTest::SmokeTest(MainWindow& window, std::wstring worker, std::wstring resultFile,
                     WindowDemoSpec spec)
    : SmokeTest(window, std::move(worker), std::wstring(), std::move(resultFile)) {
    windowDemo_ = true;
    spec_ = std::move(spec);
}

void SmokeTest::Start() {
    step_ = 0;
    sub_ = 0;
    const DWORD now = GetTickCount();
    stepDeadline_ = now + (windowDemo_ ? WINDOW_DEMO_STEP_TIMEOUT : STEP_TIMEOUT);
    overallDeadline_ = now + OVERALL_TIMEOUT;
    window_.SetTimer(TIMER_SMOKE_POLL, 100, nullptr);
}

void SmokeTest::pass(const char* name) { results_.push_back(std::string("PASS ") + name); }

void SmokeTest::fail(const std::string& reason) {
    failed_ = true;
    results_.push_back("FAIL " + reason);
    finish(true);
}

void SmokeTest::nextStep() {
    ++step_;
    sub_ = 0;
    stepDeadline_ = GetTickCount() + STEP_TIMEOUT;
}

void SmokeTest::finish(bool destroyWindow) {
    if (finished_) return;
    finished_ = true;
    if (failed_ && window_.WindowAlive()) {  // 失败诊断：附带日志尾部
        results_.push_back("--- log tail ---");
        const int total = window_.LogCount();
        for (int i = total > 30 ? total - 30 : 0; i < total; ++i)
            results_.push_back(toUtf8(window_.LogLine(i)));
    }
    writeResultFile();
    RecordSmokeExitCode(failed_ ? 1 : 0);
    if (destroyWindow) window_.DestroyForSmoke();
}

void SmokeTest::writeResultFile() { WriteSmokeResultFile(resultFile_, results_, failed_); }

void WriteSmokeResultFile(const std::wstring& path, const std::vector<std::string>& lines,
                          bool failed) {
    if (path.empty()) return;
    std::string text;
    for (const auto& line : lines) {
        text += line;
        text += "\r\n";
    }
    text += failed ? "SMOKE FAILED\r\n" : "SMOKE OK\r\n";
    HANDLE file = CreateFileW(path.c_str(), GENERIC_WRITE, 0, nullptr, CREATE_ALWAYS,
                              FILE_ATTRIBUTE_NORMAL, nullptr);
    if (file == INVALID_HANDLE_VALUE) return;
    DWORD written = 0;
    WriteFile(file, text.data(), static_cast<DWORD>(text.size()), &written, nullptr);
    CloseHandle(file);
}

void SmokeTest::OnWorkerExitObserved() {
    if (finished_) return;
    // 窗口销毁前、日志仍完整时记录停止证据；销毁钩子以此作为 PASS 前置。
    stopEvidenceOk_ = !window_.IsWorkerRunning() && window_.LastExitCode() == 0 &&
                      window_.LogContainsFrom(logMark_, L"STATE stopped");
}

void SmokeTest::OnWindowDestroyed() {
    if (finished_) return;
    finished_ = true;
    // close-while-running 只有在"关闭被推迟 + 销毁前确实正常停止"都验证过才算通过；
    // 立即销毁（回归：不等 worker）或停止证据缺失都判失败。
    if (step_ == 10 && sub_ >= 4 && stopEvidenceOk_) {
        pass("close-while-running");
        writeResultFile();
        RecordSmokeExitCode(0);
        return;
    }
    failed_ = true;
    results_.push_back("FAIL window destroyed without verified deferred close and clean stop (step " +
                       std::to_string(step_) + ", sub " + std::to_string(sub_) + ", stop-evidence " +
                       (stopEvidenceOk_ ? "ok" : "missing") + ")");
    writeResultFile();
    RecordSmokeExitCode(1);
}

void SmokeTest::Poll() {
    if (finished_) return;
    const DWORD now = GetTickCount();
    if (expired(now, overallDeadline_)) { fail("overall timeout"); return; }
    if (expired(now, stepDeadline_)) {
        fail(windowDemo_ ? "timeout in window-demo phase " + std::to_string(sub_)
                         : "timeout at step " + std::to_string(step_));
        return;
    }
    if (windowDemo_) { pollWindowDemo(); return; }
    const CString state = window_.GetText(IDC_STATE);
    switch (step_) {
    case 0:  // 演示模式开始：真实 BM_CLICK 启动真实 dm_worker
        if (sub_ == 0) {
            if (window_.CheckState(IDC_DEMO) != BST_CHECKED) { fail("demo not default"); return; }
            window_.ClickControl(IDC_START);
            sub_ = 1;
        } else if (window_.IsWorkerRunning() && state == L"执行程序已启动" &&
                   !window_.IsEnabled(IDC_START) && window_.IsEnabled(IDC_STOP) &&
                   window_.IsEnabled(IDC_PAUSE) && !window_.IsEnabled(IDC_DEMO)) {
            pass("start-demo");
            nextStep();
        }
        break;
    case 1:  // 暂停：ACK 后无新 TICK
        if (sub_ == 0) {
            if (window_.WorkerTicks() >= 3) {
                tickMark_ = window_.WorkerTicks();
                window_.ClickControl(IDC_PAUSE);
                sub_ = 1;
            }
        } else if (sub_ == 1) {
            if (state == L"已请求暂停" && window_.GetText(IDC_PAUSE) == L"继续") {
                mark_ = GetTickCount();
                sub_ = 2;
            }
        } else if (sub_ == 2) {
            if (GetTickCount() - mark_ >= 300) {
                tickMark_ = window_.WorkerTicks();
                mark_ = GetTickCount();
                sub_ = 3;
            }
        } else {
            if (GetTickCount() - mark_ >= 600) {
                if (window_.WorkerTicks() == tickMark_) { pass("pause-stable"); nextStep(); }
                else fail("ticks advanced while paused");
            }
        }
        break;
    case 2:  // 继续
        if (sub_ == 0) {
            window_.ClickControl(IDC_PAUSE);
            sub_ = 1;
        } else if (state == L"已请求继续" && window_.GetText(IDC_PAUSE) == L"暂停" &&
                   window_.WorkerTicks() > tickMark_) {
            pass("resume");
            nextStep();
        }
        break;
    case 3:  // 停止：exit0 且确实收到 STATE stopped，双条件
        if (sub_ == 0) {
            logMark_ = window_.LogCount();
            window_.ClickControl(IDC_STOP);
            mark_ = GetTickCount();
            sub_ = 1;
        } else if (!window_.IsWorkerRunning() && state == L"已停止" &&
                   window_.LastExitCode() == 0 && window_.LogContainsFrom(logMark_, L"STATE stopped")) {
            if (GetTickCount() - mark_ > 5000) fail("stop took too long");
            else { pass("stop-clean"); nextStep(); }
        }
        break;
    case 4:  // 重复启动：停止后可再启动；运行期间开始按钮不可用
        if (sub_ == 0) {
            if (!window_.IsEnabled(IDC_START)) { fail("start not re-enabled"); return; }
            window_.ClickControl(IDC_START);
            sub_ = 1;
        } else if (sub_ == 1) {
            if (window_.IsWorkerRunning() && state == L"执行程序已启动" &&
                !window_.IsEnabled(IDC_START)) {
                logMark_ = window_.LogCount();
                window_.ClickControl(IDC_STOP);
                sub_ = 2;
            }
        } else if (!window_.IsWorkerRunning() && window_.LastExitCode() == 0 &&
                   window_.LogContainsFrom(logMark_, L"STATE stopped")) {
            pass("repeat-start");
            nextStep();
        }
        break;
    case 5:  // 启动失败：执行程序路径不存在，仅提示不启动
        if (sub_ == 0) {
            savedWorkerText_ = window_.GetText(IDC_WORKER_PATH).GetString();
            logMark_ = window_.LogCount();
            window_.SetText(IDC_WORKER_PATH, (savedWorkerText_ + L"\\__missing__.exe").c_str());
            window_.ClickControl(IDC_START);
            sub_ = 1;
        } else if (!window_.IsWorkerRunning() &&
                   window_.LogContainsFrom(logMark_, L"找不到执行程序")) {
            window_.SetText(IDC_WORKER_PATH, savedWorkerText_.c_str());
            pass("start-failure");
            nextStep();
        }
        break;
    case 6:  // 字段/窗口测试切换撤销本次许可；许可文案与实际动作同步
        if (sub_ == 0) {
            if (window_.CheckState(IDC_DEMO) == BST_CHECKED) window_.ClickControl(IDC_DEMO);
            else sub_ = 1;
        } else if (sub_ == 1) {
            if (window_.CheckState(IDC_DEMO) != BST_CHECKED && window_.IsEnabled(IDC_APPROVE) &&
                window_.CheckState(IDC_APPROVE) == BST_UNCHECKED) {
                window_.SetText(IDC_DLL_PATH,
                                fixture_.empty() ? L"C:\\nonexistent\\dm.dll" : fixture_.c_str());
                window_.SetText(IDC_SHA256, SHA_PLACEHOLDER);
                window_.SetText(IDC_SDK_VERSION, FIXTURE_VERSION);
                window_.SetText(IDC_CLSID, FIXTURE_CLSID);
                window_.ClickControl(IDC_APPROVE);
                sub_ = 2;
            }
        } else if (sub_ == 2) {
            if (window_.CheckState(IDC_APPROVE) == BST_CHECKED) {
                window_.SetText(IDC_CLSID, (CString(FIXTURE_CLSID) + L"x").GetString());
                sub_ = 3;
            }
        } else if (sub_ == 3) {  // 字段变更已撤销许可；此时文案应仍是仅版本查询档
            const CString label = window_.GetText(IDC_APPROVE);
            if (window_.CheckState(IDC_APPROVE) == BST_UNCHECKED &&
                label.Find(L"查询 Ver") >= 0 && label.Find(L"绑定我们自建") < 0) {
                window_.ClickControl(IDC_WINDOW_DEMO);
                sub_ = 4;
            }
        } else if (sub_ == 4) {  // 勾选窗口测试：文案切到绑定档，且切换本身撤销许可
            const CString label = window_.GetText(IDC_APPROVE);
            if (window_.CheckState(IDC_WINDOW_DEMO) == BST_CHECKED &&
                label.Find(L"绑定我们自建") >= 0 && label.Find(L"查询 Ver") < 0 &&
                window_.CheckState(IDC_APPROVE) == BST_UNCHECKED) {
                window_.SetText(IDC_REG_CODE, L"selftest-code");
                window_.SetText(IDC_VER_INFO, L"selftest.info");
                window_.ClickControl(IDC_APPROVE);
                sub_ = 5;
            }
        } else if (sub_ == 5) {  // 许可就绪后改注册码：与其它字段一样撤销
            if (window_.CheckState(IDC_APPROVE) == BST_CHECKED) {
                window_.SetText(IDC_REG_CODE, L"selftest-code-2");
                sub_ = 6;
            }
        } else if (sub_ == 6) {  // 注册码变更已撤销；恢复后再勾许可，改 ver_info 验证同一撤销
            if (window_.CheckState(IDC_APPROVE) == BST_UNCHECKED) {
                window_.SetText(IDC_REG_CODE, L"selftest-code");
                window_.ClickControl(IDC_APPROVE);
                sub_ = 7;
            }
        } else if (sub_ == 7) {
            if (window_.CheckState(IDC_APPROVE) == BST_CHECKED) {
                window_.SetText(IDC_VER_INFO, L"selftest.info2");
                sub_ = 8;
            }
        } else if (sub_ == 8) {  // ver_info 变更撤销后取消勾选窗口测试：文案回落且许可保持撤销
            if (window_.CheckState(IDC_APPROVE) == BST_UNCHECKED) {
                window_.ClickControl(IDC_WINDOW_DEMO);
                sub_ = 9;
            }
        } else if (window_.CheckState(IDC_WINDOW_DEMO) != BST_CHECKED) {
            const CString label = window_.GetText(IDC_APPROVE);
            if (label.Find(L"查询 Ver") >= 0 && label.Find(L"绑定我们自建") < 0 &&
                window_.CheckState(IDC_APPROVE) == BST_UNCHECKED) {
                pass("approval-revocation");
                nextStep();
            }
        }
        break;
    case 7:  // 未勾选许可的 SDK 模式拒绝启动
        if (sub_ == 0) {
            window_.SetText(IDC_CLSID, FIXTURE_CLSID);  // 恢复合法字段，许可保持未勾选
            logMark_ = window_.LogCount();
            window_.ClickControl(IDC_START);
            sub_ = 1;
        } else if (!window_.IsWorkerRunning() &&
                   window_.LogContainsFrom(logMark_, L"请补齐")) {
            pass("sdk-unapproved-rejected");
            nextStep();
        }
        break;
    case 8:  // 合成 DLL 的 SDK 全链路：hash 按钮、显式许可、版本匹配、正常停止
        if (sub_ == 0) {
            if (fixture_.empty()) { fail("fixture missing"); return; }
            std::wstring expected;
            if (!Sha256File(fixture_, expected)) { fail("cannot hash fixture"); return; }
            expected_ = expected;
            window_.ClickControl(IDC_SHA_COMPUTE);
            sub_ = 1;
        } else if (sub_ == 1) {
            if (window_.GetText(IDC_SHA256).GetString() == expected_) {
                window_.ClickControl(IDC_APPROVE);
                sub_ = 2;
            }
        } else if (sub_ == 2) {
            if (window_.CheckState(IDC_APPROVE) == BST_CHECKED) {
                logMark_ = window_.LogCount();
                window_.ClickControl(IDC_START);
                sub_ = 3;
            }
        } else if (sub_ == 3) {
            if (window_.IsWorkerRunning() &&
                window_.LogContainsFrom(logMark_, L"SDK version-matched")) {
                window_.ClickControl(IDC_STOP);
                sub_ = 4;
            }
        } else if (!window_.IsWorkerRunning() && window_.LastExitCode() == 0 &&
                   window_.LogContainsFrom(logMark_, L"STATE stopped") &&
                   !window_.LogContainsFrom(logMark_, L"ERROR")) {
            pass("sdk-synthetic");
            nextStep();
        }
        break;
    case 9:  // 停止时限：very-slow 阻塞 3s，超 2s 如实提示，不强杀、不宣称成功
        if (sub_ == 0) {
            if (fixture_.empty()) { fail("fixture missing"); return; }
            SetEnvironmentVariableW(L"DM_TEST_BEHAVIOR", L"very-slow");  // 仅自测进程内，子进程继承
            window_.ClickControl(IDC_SHA_COMPUTE);
            sub_ = 1;
        } else if (sub_ == 1) {
            std::wstring expected;
            if (!Sha256File(fixture_, expected)) { fail("cannot hash fixture"); return; }
            if (window_.GetText(IDC_SHA256).GetString() == expected) {
                window_.ClickControl(IDC_APPROVE);
                sub_ = 2;
            }
        } else if (sub_ == 2) {
            if (window_.CheckState(IDC_APPROVE) == BST_CHECKED) {
                logMark_ = window_.LogCount();
                window_.ClickControl(IDC_START);
                sub_ = 3;
            }
        } else if (sub_ == 3) {
            if (window_.LogContainsFrom(logMark_, L"FIXTURE call-blocked") &&
                window_.IsWorkerRunning()) {
                window_.ClickControl(IDC_STOP);  // 在阻塞的 Ver 调用期间请求停止
                sub_ = 4;
            }
        } else if (sub_ == 4) {
            if (window_.LogContainsFrom(logMark_, L"未满足停止时限") &&
                state == L"停止超过 2 秒，仍在等待" && window_.IsWorkerRunning()) sub_ = 5;
        } else if (sub_ == 5) {
            if (!window_.IsWorkerRunning() && window_.LastExitCode() == 0 &&
                window_.LogContainsFrom(logMark_, L"STATE stopped")) {
                SetEnvironmentVariableW(L"DM_TEST_BEHAVIOR", nullptr);
                pass("stop-deadline-honest");
                nextStep();
            }
        }
        break;
    case 10: {  // 运行中关闭窗口（very-slow 阻塞期间触发）：先验证关闭被推迟，销毁前必须正常停止
        // very-slow 会打印 FIXTURE call-blocked 并阻塞 3000ms，为"关闭被推迟"提供真实忙碌窗口
        if (sub_ == 0) {
            SetEnvironmentVariableW(L"DM_TEST_BEHAVIOR", L"very-slow");
            window_.ClickControl(IDC_SHA_COMPUTE);  // 字段沿用 step 8/9 的 fixture 配置
            sub_ = 1;
        } else if (sub_ == 1) {
            std::wstring expected;
            if (!Sha256File(fixture_, expected)) { fail("cannot hash fixture"); return; }
            if (window_.GetText(IDC_SHA256).GetString() == expected) {
                window_.ClickControl(IDC_APPROVE);
                sub_ = 2;
            }
        } else if (sub_ == 2) {
            if (window_.CheckState(IDC_APPROVE) == BST_CHECKED) {
                logMark_ = window_.LogCount();
                window_.ClickControl(IDC_START);
                sub_ = 3;
            }
        } else if (sub_ == 3) {
            if (window_.LogContainsFrom(logMark_, L"FIXTURE call-blocked") &&
                window_.IsWorkerRunning()) {
                window_.RequestClose();  // 与点击 X 相同的 WM_CLOSE 路径，发生在阻塞的 Ver 调用期间
                if (finished_) return;   // 立即销毁已由 OnWindowDestroyed 判 FAIL
                const CString after = window_.GetText(IDC_STATE);
                if (window_.WindowAlive() && window_.IsWorkerRunning() &&
                    after == L"正在停止并等待释放") {
                    stopEvidenceOk_ = false;
                    sub_ = 4;  // 关闭被推迟：窗口仍在、worker 仍在协作停止，未被强杀
                } else {
                    fail("close not deferred while worker busy");
                }
            }
        }
        // sub_ == 4：等待 worker 正常退出（OnWorkerExitObserved 记录证据）后窗口才销毁
        break;
    }
    default:
        fail("unknown step");
        break;
    }
}

void SmokeTest::pollWindowDemo() {
    // 启动被真实控件校验拒绝（字段/许可缺失、路径错误）时快速失败，不等到超时
    if (sub_ >= 4 && !window_.IsWorkerRunning() &&
        (window_.LogContainsFrom(logMark_, L"请补齐") ||
         window_.LogContainsFrom(logMark_, L"找不到执行程序") ||
         window_.LogContainsFrom(logMark_, L"执行程序错误"))) {
        fail("window-demo start rejected by UI validation");
        return;
    }
    switch (sub_) {
    case 0:  // 退出默认演示模式（真实 BM_CLICK），进入 SDK 模式
        window_.ClickControl(IDC_DEMO);
        sub_ = 1;
        break;
    case 1:  // 预填真实字段并勾选绑定自建测试窗口；字段变化会撤销许可，许可须最后勾
        if (window_.CheckState(IDC_DEMO) != BST_CHECKED && window_.IsEnabled(IDC_WINDOW_DEMO)) {
            window_.SetText(IDC_DLL_PATH, spec_.dll.c_str());
            window_.SetText(IDC_SHA256, spec_.sha256.c_str());
            window_.SetText(IDC_SDK_VERSION, spec_.version.c_str());
            window_.SetText(IDC_CLSID, spec_.clsid.c_str());
            window_.SetText(IDC_REG_CODE, spec_.regCode.c_str());
            window_.SetText(IDC_VER_INFO, spec_.regInfo.c_str());
            window_.ClickControl(IDC_WINDOW_DEMO);
            sub_ = 2;
        }
        break;
    case 2:
        if (window_.CheckState(IDC_WINDOW_DEMO) == BST_CHECKED) {
            window_.ClickControl(IDC_APPROVE);
            sub_ = 3;
        }
        break;
    case 3:
        if (window_.CheckState(IDC_APPROVE) == BST_CHECKED) {
            logMark_ = window_.LogCount();
            logScan_ = logMark_;
            markerIndex_ = 0;
            window_.ClickControl(IDC_START);
            // 注册码已随启动一次性发往私有管道；UI 清空密码框后尽力擦除自测副本
            if (!spec_.regCode.empty())
                SecureZeroMemory(&spec_.regCode[0], spec_.regCode.size() * sizeof(wchar_t));
            spec_.regCode.clear();
            if (!spec_.regInfo.empty())
                SecureZeroMemory(&spec_.regInfo[0], spec_.regInfo.size() * sizeof(wchar_t));
            spec_.regInfo.clear();
            sub_ = 4;
        }
        break;
    default:  // 观察 DEMO 终态序列 + STATE stopped + exit0
        observeWindowDemoLog();
        if (finished_) return;
        if (!window_.IsWorkerRunning()) {
            if (window_.LastExitCode() != 0) {
                fail("window-demo worker exit code " + std::to_string(window_.LastExitCode()));
            } else if (markerIndex_ < kWindowDemoMarkerCount) {
                fail("window-demo worker exited after " + std::to_string(markerIndex_) + "/" +
                     std::to_string(kWindowDemoMarkerCount) + " DEMO markers");
            } else if (window_.LogContainsFrom(logMark_, L"STATE stopped")) {
                pass("window-demo-e2e");
                // 附上实际观察到的完整序列作为通过证据
                for (int i = logMark_; i < window_.LogCount(); ++i)
                    results_.push_back(toUtf8(window_.LogLine(i)));
                finish(true);
            }
        }
        break;
    }
}

void SmokeTest::observeWindowDemoLog() {
    while (logScan_ < window_.LogCount()) {
        const CString line = window_.LogLine(logScan_++);
        if (line.Find(L"ERROR") >= 0) { fail("window-demo error line: " + toUtf8(line)); return; }
        if (markerIndex_ >= kWindowDemoMarkerCount) continue;
        if (line.Find(kWindowDemoMarkers[markerIndex_]) >= 0) { ++markerIndex_; continue; }
        for (int later = markerIndex_ + 1; later < kWindowDemoMarkerCount; ++later) {
            if (line.Find(kWindowDemoMarkers[later]) >= 0) {
                fail("window-demo marker out of order: expected " +
                     toUtf8(CStringW(kWindowDemoMarkers[markerIndex_])) + " before " +
                     toUtf8(CStringW(kWindowDemoMarkers[later])));
                return;
            }
        }
    }
}

} // namespace dm
