#include "control.h"

namespace dm {

void Control::pause() {
    std::lock_guard<std::mutex> lock(mutex_);
    paused_ = true;
    changed_.notify_all();
}

void Control::resume() {
    std::lock_guard<std::mutex> lock(mutex_);
    paused_ = false;
    changed_.notify_all();
}

void Control::stop() {
    std::lock_guard<std::mutex> lock(mutex_);
    stopped_ = true;
    changed_.notify_all();
}

bool Control::checkpoint() {
    std::unique_lock<std::mutex> lock(mutex_);
    changed_.wait(lock, [this] { return stopped_ || !paused_; });
    return !stopped_;
}

void Control::waitFor(std::chrono::milliseconds duration) {
    std::unique_lock<std::mutex> lock(mutex_);
    changed_.wait_for(lock, duration, [this] { return stopped_ || paused_; });
}

} // namespace dm
