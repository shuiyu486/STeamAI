#pragma once

#include <chrono>
#include <condition_variable>
#include <mutex>

namespace dm {

class Control {
public:
    void pause();
    void resume();
    void stop();
    // 返回 false 后执行单元不得开始下一步；等待可被暂停/停止唤醒。
    bool checkpoint();
    void waitFor(std::chrono::milliseconds duration);

private:
    std::mutex mutex_;
    std::condition_variable changed_;
    bool paused_ = false;
    bool stopped_ = false;
};

} // namespace dm
