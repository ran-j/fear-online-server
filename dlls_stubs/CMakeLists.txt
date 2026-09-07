cmake_minimum_required(VERSION 3.21)

project(FearOnlineStubs LANGUAGES C)

if(NOT MSVC)
    message(FATAL_ERROR
        "Build the stubs with the MSVC toolchain (Visual Studio / Build Tools).\n"
        "The ZNetwork proxy forwards exports via MSVC '/export' linker pragmas "
        "that other compilers do not support.")
endif()

if(NOT CMAKE_SIZEOF_VOID_P EQUAL 4)
    message(FATAL_ERROR
        "The stubs must be built as 32-bit (x86); the client will not load 64-bit "
        "DLLs.\nReconfigure with a Win32 generator platform, e.g.:\n"
        "  cmake -G \"Visual Studio 17 2022\" -A Win32 -S . -B build")
endif()

set(CMAKE_RUNTIME_OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}/dist")

add_subdirectory(zlauncher_stub)
add_subdirectory(znetwork_stub)
