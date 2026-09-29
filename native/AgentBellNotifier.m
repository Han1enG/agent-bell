#import <Cocoa/Cocoa.h>

int main(int argc, const char *argv[]) {
    @autoreleasepool {
        if (argc < 4) {
            fprintf(stderr, "usage: AgentBellNotifier <title> <subtitle> <message>\n");
            return 2;
        }

        NSUserNotification *notification = [[NSUserNotification alloc] init];
        notification.title = [NSString stringWithUTF8String:argv[1]];
        notification.subtitle = [NSString stringWithUTF8String:argv[2]];
        notification.informativeText = [NSString stringWithUTF8String:argv[3]];

        [[NSUserNotificationCenter defaultUserNotificationCenter] deliverNotification:notification];
        [[NSRunLoop currentRunLoop] runUntilDate:[NSDate dateWithTimeIntervalSinceNow:0.5]];
    }
    return 0;
}
