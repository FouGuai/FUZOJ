#include <seccomp.h>
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <sys/prctl.h>
#include <sys/socket.h>

int main() {
    if (prctl(PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0)) {
        perror("prctl");
        return 1;
    }

    scmp_filter_ctx ctx = seccomp_init(SCMP_ACT_ALLOW);
    if (!ctx) {
        perror("seccomp_init");
        return 1;
    }

    // 禁用 socket
    if (seccomp_rule_add(ctx, SCMP_ACT_KILL, SCMP_SYS(socket), 0) < 0) {
        perror("seccomp_rule_add");
        return 1;
    }

    if (seccomp_load(ctx) < 0) {
        perror("seccomp_load");
        return 1;
    }

    seccomp_release(ctx);

    printf("Seccomp loaded successfully!\n");

    // 测试：正常的 write
    write(1, "hello\n", 6);

    // 测试：调用 socket 应该直接杀死进程
    socket(AF_INET, SOCK_STREAM, 0);

    return 0;
}