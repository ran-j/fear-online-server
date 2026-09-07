# ZNetwork.dll — a logging proxy over the real ZNetwork_orig.dll.
add_library(znetwork_stub SHARED
    proxy.c
    forwards.h
)

set_target_properties(znetwork_stub PROPERTIES
    OUTPUT_NAME "ZNetwork"
    PREFIX ""            # ZNetwork.dll, not libZNetwork.dll
)
