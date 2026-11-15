cmake_minimum_required(VERSION 3.20)
project(DmStarter VERSION 0.1.0 LANGUAGES CXX)

if(NOT WIN32)
    message(FATAL_ERROR "此工程仅支持 Windows 10/11。")
endif()
set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)
# 统一静态 CRT：分发目录不依赖 VC 运行库。
set(CMAKE_MSVC_RUNTIME_LIBRARY "MultiThreaded$<$<CONFIG:Debug>:Debug>")
add_compile_definitions(UNICODE _UNICODE WIN32_LEAN_AND_MEAN NOMINMAX)
if(MSVC)
    add_compile_options(/W4 /utf-8 /permissive- /EHsc)
endif()

option(DM_BUILD_WORKER "构建与大漠 DLL 同位数的 C++ 执行程序" ON)
option(DM_BUILD_GUI "构建 MFC 界面；需要 VS2019 的 MFC 组件（静态 MFC + 静态 CRT）" ON)
include(CTest)

if(DM_BUILD_WORKER)
    add_library(dm_core STATIC src/control.cpp src/dm_session.cpp src/license_input.cpp)
    target_include_directories(dm_core PUBLIC src)
    target_link_libraries(dm_core PUBLIC ole32 oleaut32 bcrypt)
    add_executable(dm_worker src/worker_main.cpp src/window_demo.cpp src/mfc_worker_process.cpp)
    target_link_libraries(dm_worker PRIVATE dm_core user32)
    add_executable(dm_demo_target WIN32 src/demo_target.cpp)
    target_link_libraries(dm_demo_target PRIVATE user32 gdi32 shell32)
    install(TARGETS dm_worker dm_demo_target RUNTIME DESTINATION .)
    # 窄 SDK 冒烟：仅显式运行（真实 DLL 动作不进默认测试）。
    add_executable(dm_sdk_smoke tests/sdk_smoke.cpp)
    target_link_libraries(dm_sdk_smoke PRIVATE dm_core user32 gdi32)
    if(BUILD_TESTING)
        find_package(Python3 REQUIRED COMPONENTS Interpreter)
        add_library(synthetic_dm SHARED tests/synthetic_dm.cpp tests/synthetic_dm.def)
        target_link_libraries(synthetic_dm PRIVATE ole32 oleaut32)
        add_test(NAME worker_lifecycle
            COMMAND "${Python3_EXECUTABLE}" -B "${CMAKE_CURRENT_SOURCE_DIR}/tests/test_worker.py"
                --worker "$<TARGET_FILE:dm_worker>" --fixture "$<TARGET_FILE:synthetic_dm>"
                --smoke "$<TARGET_FILE:dm_sdk_smoke>" --target "$<TARGET_FILE:dm_demo_target>")
        set_tests_properties(worker_lifecycle PROPERTIES TIMEOUT 90)
        add_test(NAME demo_target_protocol
            COMMAND "${Python3_EXECUTABLE}" -B "${CMAKE_CURRENT_SOURCE_DIR}/tests/test_demo_target.py"
                --target "$<TARGET_FILE:dm_demo_target>")
        set_tests_properties(demo_target_protocol PROPERTIES TIMEOUT 60)
    endif()
endif()

if(DM_BUILD_GUI)
    set(CMAKE_MFC_FLAG 1)  # 静态链接 MFC
    add_executable(dm_console WIN32
        src/mfc_main.cpp src/license_input.cpp
        src/mfc_window.cpp src/mfc_window.h
        src/mfc_worker_process.cpp src/mfc_worker_process.h
        src/mfc_selftest.cpp src/mfc_selftest.h
        res/resource.h res/dm_console.rc)
    target_include_directories(dm_console PRIVATE src res)
    target_link_libraries(dm_console PRIVATE bcrypt)
    if(MSVC)
        # 清单由 res/dm_console.rc 嵌入，关闭 linker 自动嵌入避免资源重复
        target_link_options(dm_console PRIVATE /MANIFEST:NO)
    endif()
    install(TARGETS dm_console RUNTIME DESTINATION .)
    if(BUILD_TESTING AND DM_BUILD_WORKER)
        add_test(NAME gui_smoke
            COMMAND "${Python3_EXECUTABLE}" -B "${CMAKE_CURRENT_SOURCE_DIR}/tests/test_mfc.py"
                --console "$<TARGET_FILE:dm_console>" --worker "$<TARGET_FILE:dm_worker>"
                --fixture "$<TARGET_FILE:synthetic_dm>")
        set_tests_properties(gui_smoke PROPERTIES TIMEOUT 180)
    endif()
endif()
