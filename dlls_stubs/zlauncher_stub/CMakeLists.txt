add_library(zlauncher_stub SHARED
    zlauncher_stub.c
    ZLauncher.def
)

set_target_properties(zlauncher_stub PROPERTIES
    OUTPUT_NAME "ZLauncher"
    PREFIX ""
)
