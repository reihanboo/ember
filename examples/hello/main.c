#define _POSIX_C_SOURCE 200809L

#include <stdio.h>

#ifdef _WIN32
#include <windows.h>
#else
#include <unistd.h>
#endif

int main(void) {
    unsigned long counter = 1;

    for (;;) {
        printf("Hello from Ember! Counter: %lu\n", counter++);
        fflush(stdout);
#ifdef _WIN32
        Sleep(1000);
#else
        sleep(1);
#endif
    }
}
