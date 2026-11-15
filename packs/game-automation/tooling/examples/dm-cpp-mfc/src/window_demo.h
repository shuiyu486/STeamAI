#pragma once

#include "control.h"
#include "dm_session.h"

#include <filesystem>
#include <string>

namespace dm {

// 只启动本工程的独立测试目标，不接受外部窗口句柄。
void runWindowDemo(Session& session, Control& control, const std::filesystem::path& target,
                   bool hidden, void (*report)(const std::string&));

} // namespace dm
