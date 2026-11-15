#include "control.h"
#include "dm_session.h"
#include "window_demo.h"
#include "license_input.h"

#include <algorithm>
#include <atomic>
#include <chrono>
#include <iostream>
#include <map>
#include <memory>
#include <mutex>
#include <stdexcept>
#include <thread>

namespace {

std::mutex outputMutex;

void report(const std::string& text) {
    std::lock_guard<std::mutex> lock(outputMutex);
    std::cout << text << std::endl;
}

struct Options {
    bool demo = true;
    bool windowDemo = false;
    bool hiddenTarget = false;
    bool regApproved = false;
    bool licenseStdin = false;
    std::filesystem::path target;
    dm::SdkConfig sdk;
};

Options parse(int argc, wchar_t** argv) {
    if (argc == 2 && std::wstring(argv[1]) == L"--demo") return {};
    if (argc < 10 || std::wstring(argv[1]) != L"--sdk-approved")
        throw std::runtime_error("Usage: dm_worker --demo OR --sdk-approved --dll PATH --sha256 HASH --version VERSION --clsid GUID [--window-demo --target PATH --reg-approved --license-stdin [--hidden-target]]");
    Options result;
    result.demo = false;
    std::map<std::wstring, std::wstring> fields;
    for (int i = 2; i < argc; ++i) {
        const std::wstring key(argv[i]);
        if (key == L"--window-demo" || key == L"--hidden-target" || key == L"--reg-approved" || key == L"--license-stdin") {
            bool& flag = key == L"--window-demo" ? result.windowDemo : key == L"--hidden-target" ? result.hiddenTarget :
                         key == L"--reg-approved" ? result.regApproved : result.licenseStdin;
            if (flag) throw std::runtime_error("Duplicate demo flag");
            flag = true;
            continue;
        }
        if (key != L"--dll" && key != L"--sha256" && key != L"--version" && key != L"--clsid" && key != L"--target")
            throw std::runtime_error("Unknown SDK option");
        if (++i == argc || !fields.emplace(key, argv[i]).second)
            throw std::runtime_error("Missing or duplicate SDK option");
    }
    if (!fields.count(L"--dll") || !fields.count(L"--sha256") || !fields.count(L"--version") || !fields.count(L"--clsid"))
        throw std::runtime_error("Missing SDK option");
    if (result.windowDemo != (fields.count(L"--target") != 0) || (result.hiddenTarget && !result.windowDemo))
        throw std::runtime_error("Target options require --window-demo and an explicit --target");
    if (result.windowDemo != result.regApproved || result.windowDemo != result.licenseStdin)
        throw std::runtime_error("Window demo requires --reg-approved --license-stdin; version-only mode does not register");
    if (result.windowDemo) result.target = fields.at(L"--target");
    result.sdk.dll = fields.at(L"--dll");
    const auto hash = fields.at(L"--sha256");
    if (!std::all_of(hash.begin(), hash.end(), [](wchar_t c) { return c >= 0 && c <= 127; }))
        throw std::runtime_error("SHA-256 must be ASCII hexadecimal");
    for (wchar_t c : hash) result.sdk.sha256.push_back(static_cast<char>(c));
    result.sdk.version = fields.at(L"--version");
    result.sdk.clsid = fields.at(L"--clsid");
    return result;
}

} // namespace

int wmain(int argc, wchar_t** argv) {
    Options options;
    try { options = parse(argc, argv); }
    catch (const std::exception& e) { report(std::string("ERROR ") + e.what()); return 2; }

    const HANDLE input = GetStdHandle(STD_INPUT_HANDLE);
    if (input == INVALID_HANDLE_VALUE || GetFileType(input) != FILE_TYPE_PIPE) {
        report("ERROR stdin must be a private parent pipe");
        return 2;
    }
    dm::LicenseInput license;
    try { if (options.windowDemo) license = dm::readLicenseInput(); }
    catch (const std::exception& e) { report(std::string("ERROR ") + e.what()); return 2; }
    dm::Control control;
    std::atomic<bool> finished{false};
    int result = 0;
    bool protocolError = false;
    std::thread execution([&] {
        try {
            {
                std::unique_ptr<dm::Session> session;
                if (control.checkpoint() && !options.demo) {
                    report("STATE loading");
                    session = std::make_unique<dm::Session>(options.sdk);
                    report("SDK version-matched");
                    if (options.windowDemo && control.checkpoint()) {
                        session->registerLicense(license.code, license.info);
                        license.clear();
                        report("SDK license-registered");
                    }
                }
                if (control.checkpoint()) report(options.demo ? "STATE running-demo" : "STATE running-sdk");
                if (options.windowDemo && session && control.checkpoint()) {
                    dm::runWindowDemo(*session, control, options.target, options.hiddenTarget, report);
                } else {
                    while (control.checkpoint()) {
                        // 未勾选窗口测试时只有心跳，不操作任何窗口。
                        report("TICK");
                        control.waitFor(std::chrono::milliseconds(100));
                    }
                }
            }
        } catch (const std::exception& e) {
            report(std::string("ERROR ") + e.what());
            result = 1;
        } catch (...) {
            report("ERROR unexpected worker failure");
            result = 1;
        }
        license.clear();
        finished.store(true);
    });

    // 只读父进程私有 stdin 管道；不接受任意方法名、脚本或目标句柄。
    {
        std::string line;
        bool done = false;
        while (!done && !finished.load()) {
            DWORD available = 0;
            if (!PeekNamedPipe(input, nullptr, 0, nullptr, &available, nullptr)) break;
            if (!available) { Sleep(10); continue; }
            char c = 0;
            DWORD count = 0;
            if (!ReadFile(input, &c, 1, &count, nullptr) || count == 0) break;
            if (c == '\r') continue;
            if (c == '\n') {
                if (line == "pause") { control.pause(); report("ACK pause-requested"); }
                else if (line == "resume") { control.resume(); report("ACK resume-requested"); }
                else if (line == "stop") { control.stop(); report("ACK stop-requested"); done = true; }
                else { report("ERROR invalid command; stopping"); protocolError = true; done = true; }
                line.clear();
            } else if (line.size() < 64) line += c;
            else { report("ERROR command too long; stopping"); protocolError = true; done = true; }
        }
        control.stop();
    }
    execution.join();
    if (result == 0) report("STATE stopped");
    return protocolError ? 2 : result;
}
