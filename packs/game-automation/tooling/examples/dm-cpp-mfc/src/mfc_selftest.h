#pragma once

#include "mfc_window.h"

#include <string>
#include <vector>

namespace dm {

// --window-demo-smoke 隐藏验收入口的显式输入：DLL、校验、版本与 CLSID 全部来自命令行，
// 缺任一项在创建窗口前即拒绝；不自动落到合成夹具，也不显示可见窗口。
// 注册码只经本进程私有 stdin 读入并预填密码框，再由 UI 一次性发往 worker 私有管道，
// 绝不进入日志、命令行或结果文件。
struct WindowDemoSpec {
    std::wstring dll;
    std::wstring sha256;
    std::wstring version;
    std::wstring clsid;
    std::wstring regCode;
    std::wstring regInfo;
};

// 隐藏窗口内的自测：全部走与用户相同的 BM_CLICK / WM_CLOSE / 定时器消息路径，
// 驱动真实 dm_worker.exe；SDK 只用合成 DLL，绝不加载用户大漠或触碰注册表。
// window-demo 模式复用同一套结果文件/退出码/收尾机制，只观察 worker 的
// DEMO 终态序列与 exit0，不复制窗口驱动逻辑。
class SmokeTest {
public:
    // 经典 11 检查点自测（--ui-smoke）
    SmokeTest(MainWindow& window, std::wstring worker, std::wstring fixture, std::wstring resultFile);
    // 绑定自建测试窗口端到端（--window-demo-smoke）；DLL 由调用方显式给定
    SmokeTest(MainWindow& window, std::wstring worker, std::wstring resultFile, WindowDemoSpec spec);

    void Start();
    void Poll();                // 100ms 窗口定时器驱动
    void OnWorkerExitObserved();  // handleWorkerExit 在销毁窗口前的回调（日志仍完整）
    void OnWindowDestroyed();   // 窗口销毁回调（结束判定）

private:
    void pass(const char* name);
    void fail(const std::string& reason);
    void nextStep();
    void finish(bool destroyWindow);
    void writeResultFile();
    void pollWindowDemo();
    void observeWindowDemoLog();

    MainWindow& window_;
    std::wstring workerPath_;
    std::wstring fixture_;
    std::wstring resultFile_;
    std::vector<std::string> results_;
    bool failed_ = false;
    bool finished_ = false;
    int step_ = 0;
    int sub_ = 0;
    DWORD stepDeadline_ = 0;
    DWORD overallDeadline_ = 0;
    DWORD mark_ = 0;
    int logMark_ = 0;
    unsigned long tickMark_ = 0;
    std::wstring savedWorkerText_;
    std::wstring expected_;
    bool stopEvidenceOk_ = false;  // 关闭场景：销毁前确实观察到 exit0 + STATE stopped
    bool windowDemo_ = false;      // --window-demo-smoke 模式
    WindowDemoSpec spec_;
    int logScan_ = 0;              // window-demo：已核对到的日志行号
    int markerIndex_ = 0;          // window-demo：下一个期望的 DEMO 标记
};

// 结果文件写入由自测与 mfc_main 的参数拒绝路径共用。
void WriteSmokeResultFile(const std::wstring& path, const std::vector<std::string>& lines,
                          bool failed);

// 由 mfc_main 提供存储；自测结束时写入进程退出码。
void RecordSmokeExitCode(int code);

} // namespace dm
