#include <afxwin.h>

#include "license_input.h"
#include "mfc_selftest.h"
#include "mfc_window.h"

#include <shellapi.h>

#include <set>
#include <stdexcept>
#include <string>
#include <vector>

namespace {

int smokeExitCode = 0;

} // namespace

namespace dm {

void RecordSmokeExitCode(int code) { smokeExitCode = code; }

} // namespace dm

namespace {

struct Options {
    bool smoke = false;
    bool windowDemoSmoke = false;
    bool sdkApproved = false;
    bool regApproved = false;
    bool licenseStdin = false;
    bool hasArguments = false;
    std::wstring worker;
    std::wstring fixture;
    std::wstring resultFile;
    std::wstring dll, sha256, version, clsid;
};

Options parseOptions(bool& rejected, std::string& rejectReason) {
    Options options;
    int argc = 0;
    bool unknownArgument = false, duplicateOption = false, missingValue = false;
    std::wstring missingFor;
    std::set<std::wstring> seen;
    if (wchar_t** argv = CommandLineToArgvW(GetCommandLineW(), &argc)) {
        options.hasArguments = argc > 1;
        for (int i = 1; i < argc; ++i) {
            const std::wstring argument(argv[i]);
            // 下项以 -- 开头或缺省时不当作值：宁可拒绝，也不把模式标志吞成前一项的值
            auto value = [&](const std::wstring& name) {
                if (i + 1 >= argc || wcsncmp(argv[i + 1], L"--", 2) == 0) {
                    missingValue = true;
                    if (missingFor.empty()) missingFor = name;
                    return std::wstring();
                }
                return std::wstring(argv[++i]);
            };
            std::wstring* target = nullptr;
            if (argument == L"--ui-smoke") options.smoke = true;
            else if (argument == L"--window-demo-smoke") options.windowDemoSmoke = true;
            else if (argument == L"--sdk-approved") options.sdkApproved = true;
            else if (argument == L"--reg-approved") options.regApproved = true;
            else if (argument == L"--license-stdin") options.licenseStdin = true;
            else if (argument == L"--worker") target = &options.worker;
            else if (argument == L"--fixture") target = &options.fixture;
            else if (argument == L"--dll") target = &options.dll;
            else if (argument == L"--sha256") target = &options.sha256;
            else if (argument == L"--version") target = &options.version;
            else if (argument == L"--clsid") target = &options.clsid;
            else if (argument == L"--result-file") target = &options.resultFile;
            else unknownArgument = true;
            if (target != nullptr) *target = value(argument);
            if (!seen.insert(argument).second) duplicateOption = true;
        }
        LocalFree(argv);
    }
    auto reject = [&](const std::string& reason) {
        rejected = true;
        rejectReason = reason;
    };
    // 只要出现命令行参数，就必须完整落在合法的隐藏模式上；任何解析异常都拒绝，
    // 绝不吞掉模式标志后退回可见界面。无参数双击仍是普通可见启动。
    if (options.hasArguments) {
        if (unknownArgument) reject("unknown argument present");
        else if (missingValue) {
            std::string option;  // 已识别的选项名仅包含 ASCII
            for (wchar_t c : missingFor) option.push_back(static_cast<char>(c));
            reject("missing value for option " + option);
        } else if (duplicateOption) reject("duplicate option");
        else if (!options.smoke && !options.windowDemoSmoke)
            reject("no valid smoke mode selected");
        else if (options.windowDemoSmoke) {
            // --window-demo-smoke 是真实 DLL 的隐藏验收入口：明确许可参数必须齐、
            // 无未知/重复参数；不降级为可见窗口、演示模式或合成夹具。
            const char* reason = nullptr;
            if (options.smoke || !options.fixture.empty())
                reason = "--ui-smoke/--fixture cannot combine with --window-demo-smoke";
            else if (!options.sdkApproved) reason = "missing --sdk-approved";
            else if (!options.regApproved) reason = "missing --reg-approved";
            else if (!options.licenseStdin) reason = "missing --license-stdin";
            else if (options.dll.empty()) reason = "missing --dll";
            else if (options.sha256.empty()) reason = "missing --sha256";
            else if (options.version.empty()) reason = "missing --version";
            else if (options.clsid.empty()) reason = "missing --clsid";
            else if (options.resultFile.empty()) reason = "missing --result-file";
            if (reason != nullptr) reject(reason);
        }
    }
    return options;
}

class DmConsoleApp : public CWinApp {
public:
    BOOL InitInstance() override;
    int ExitInstance() override { return smokeExitCode; }
};

BOOL DmConsoleApp::InitInstance() {
    CWinApp::InitInstance();
    bool argumentsRejected = false;
    std::string rejectReason;
    const Options options = parseOptions(argumentsRejected, rejectReason);
    if (argumentsRejected) {
        // 参数解析异常或缺合法隐藏模式：不创建窗口（更不会显示），只如实拒绝。
        const std::vector<std::string> lines{"FAIL arguments rejected: " + rejectReason};
        dm::WriteSmokeResultFile(options.resultFile, lines, true);
        dm::RecordSmokeExitCode(1);
        return FALSE;
    }
    dm::LicenseInput license;  // 注册码从本进程私有 stdin 读入，错误只回泛化原因
    if (options.windowDemoSmoke) {
        try {
            license = dm::readLicenseInput();
        } catch (const std::exception& e) {
            const std::vector<std::string> lines{
                "FAIL window-demo-smoke rejected: license input (" + std::string(e.what()) + ")"};
            dm::WriteSmokeResultFile(options.resultFile, lines, true);
            dm::RecordSmokeExitCode(1);
            return FALSE;
        }
    }
    auto* window = new dm::MainWindow();  // 主框架按 MFC 惯例自删除
    if (!window->Create(nullptr, L"大漠 C++/MFC 基础工程", WS_OVERLAPPEDWINDOW,
                        CRect(0, 0, 800, 620))) {
        license.clear();
        delete window;
        return FALSE;
    }
    m_pMainWnd = window;
    if (options.windowDemoSmoke) {
        // 隐藏验收入口：预填真实控件/勾选并 BM_CLICK 开始，观察 DEMO 终态与 exit0 后退出。
        dm::WindowDemoSpec spec{options.dll, options.sha256, options.version, options.clsid,
                                license.code, license.info};
        license.clear();
        window->RunWindowDemoSmoke(options.worker, spec, options.resultFile);
        // 尽力擦除本地副本（自测内另一份在发送后自行擦除），不作完美擦除宣称
        if (!spec.regCode.empty())
            SecureZeroMemory(spec.regCode.data(), spec.regCode.size() * sizeof(wchar_t));
        spec.regCode.clear();
        if (!spec.regInfo.empty())
            SecureZeroMemory(spec.regInfo.data(), spec.regInfo.size() * sizeof(wchar_t));
        spec.regInfo.clear();
    } else if (options.smoke) {
        // 隐藏自测：不显示窗口，只走真实控件消息路径 + 真实 dm_worker。
        window->RunSmoke(options.worker, options.fixture, options.resultFile);
    } else {
        window->ShowWindow(m_nCmdShow);
        window->UpdateWindow();
    }
    return TRUE;
}

DmConsoleApp theApp;

} // namespace
