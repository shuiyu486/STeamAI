#include "license_input.h"

#include <windows.h>
#include <chrono>
#include <stdexcept>

namespace dm {

void LicenseInput::clear() noexcept {
    if (!code.empty()) SecureZeroMemory(code.data(), code.size() * sizeof(wchar_t));
    if (!info.empty()) SecureZeroMemory(info.data(), info.size() * sizeof(wchar_t));
    code.clear();
    info.clear();
}

void validateLicenseInput(const LicenseInput& license) {
    if (license.code.empty() || license.code.size() > 512 || license.info.size() > 40)
        throw std::runtime_error("Invalid license field length");
    for (wchar_t c : license.code)
        if (c < 0x20 || c == 0x7f) throw std::runtime_error("Invalid license code character");
    for (wchar_t c : license.info)
        if (!((c >= L'a' && c <= L'z') || (c >= L'A' && c <= L'Z') ||
              (c >= L'0' && c <= L'9') || c == L'.'))
            throw std::runtime_error("Invalid license version-info character");
}

std::string licenseLine(const LicenseInput& license) {
    validateLicenseInput(license);
    std::wstring wide = L"LICENSE\t" + license.code + L"\t" + license.info;
    const int count = WideCharToMultiByte(CP_UTF8, WC_ERR_INVALID_CHARS, wide.data(),
                                         static_cast<int>(wide.size()), nullptr, 0, nullptr, nullptr);
    if (count == 0) {
        SecureZeroMemory(wide.data(), wide.size() * sizeof(wchar_t));
        throw std::runtime_error("Cannot encode license input");
    }
    std::string line(count, '\0');
    WideCharToMultiByte(CP_UTF8, WC_ERR_INVALID_CHARS, wide.data(), static_cast<int>(wide.size()),
                        line.data(), count, nullptr, nullptr);
    SecureZeroMemory(wide.data(), wide.size() * sizeof(wchar_t));
    return line;
}

LicenseInput readLicenseInput() {
    const HANDLE input = GetStdHandle(STD_INPUT_HANDLE);
    if (!input || input == INVALID_HANDLE_VALUE || GetFileType(input) != FILE_TYPE_PIPE)
        throw std::runtime_error("License input requires a private parent pipe");
    std::string line;
    const auto deadline = std::chrono::steady_clock::now() + std::chrono::seconds(5);
    try {
        for (;;) {
            if (std::chrono::steady_clock::now() >= deadline)
                throw std::runtime_error("License input timed out");
            DWORD available = 0;
            if (!PeekNamedPipe(input, nullptr, 0, nullptr, &available, nullptr))
                throw std::runtime_error("License pipe closed before initialization");
            if (!available) { Sleep(10); continue; }
            char c = 0;
            DWORD read = 0;
            if (!ReadFile(input, &c, 1, &read, nullptr) || read != 1)
                throw std::runtime_error("Cannot read license initialization");
            if (c == '\n') break;
            if (c == '\r' || line.size() >= 4096)
                throw std::runtime_error("Invalid license initialization line");
            line.push_back(c);
        }
        if (line.rfind("LICENSE\t", 0) != 0)
            throw std::runtime_error("Missing license initialization marker");
        const size_t separator = line.find('\t', 8);
        if (separator == std::string::npos || line.find('\t', separator + 1) != std::string::npos)
            throw std::runtime_error("Invalid license initialization fields");
        const int count = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, line.data(),
                                              static_cast<int>(line.size()), nullptr, 0);
        if (count == 0) throw std::runtime_error("License input must be UTF-8");
        std::wstring wide(count, L'\0');
        MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, line.data(), static_cast<int>(line.size()),
                            wide.data(), count);
        const size_t split = wide.find(L'\t', 8);
        LicenseInput license{wide.substr(8, split - 8), wide.substr(split + 1)};
        SecureZeroMemory(wide.data(), wide.size() * sizeof(wchar_t));
        SecureZeroMemory(line.data(), line.size());
        try { validateLicenseInput(license); }
        catch (...) { license.clear(); throw; }
        return license;
    } catch (...) {
        if (!line.empty()) SecureZeroMemory(line.data(), line.size());
        throw;
    }
}

} // namespace dm
