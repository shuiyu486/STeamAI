#pragma once

#include <string>

namespace dm {

struct LicenseInput {
    std::wstring code;
    std::wstring info;
    void clear() noexcept;
};

// 只读当前进程私有 stdin 管道的一行初始化消息，最多等五秒，不回显原文。
LicenseInput readLicenseInput();
void validateLicenseInput(const LicenseInput& license);
std::string licenseLine(const LicenseInput& license); // 返回不带换行的 UTF-8 行，由私有管道发送。

} // namespace dm
