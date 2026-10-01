#import <AppKit/AppKit.h>
#import <UserNotifications/UserNotifications.h>
#import <string.h>

@interface BellDelegate : NSObject <UNUserNotificationCenterDelegate>
@property(nonatomic, assign) BOOL handled;
@end

static NSString *AS(NSString *value) {
    NSString *s = value ?: @"";
    s = [s stringByReplacingOccurrencesOfString:@"\\" withString:@"\\\\"];
    s = [s stringByReplacingOccurrencesOfString:@"\"" withString:@"\\\""];
    s = [s stringByReplacingOccurrencesOfString:@"\n" withString:@"\\n"];
    s = [s stringByReplacingOccurrencesOfString:@"\r" withString:@"\\r"];
    return [NSString stringWithFormat:@"\"%@\"", s];
}

static NSString *ShellQuote(NSString *value) {
    NSString *escaped = [value stringByReplacingOccurrencesOfString:@"'" withString:@"'\\''"];
    return [NSString stringWithFormat:@"'%@'", escaped];
}

static void DebugLog(NSString *message) {
    NSString *directory = [NSHomeDirectory() stringByAppendingPathComponent:@"Library/Logs/AgentBell"];
    [[NSFileManager defaultManager] createDirectoryAtPath:directory withIntermediateDirectories:YES attributes:@{NSFilePosixPermissions: @0700} error:nil];
    NSString *path = [directory stringByAppendingPathComponent:@"agentbell.log"];
    NSString *line = [NSString stringWithFormat:@"%@ %@\n", [NSDate date], message];
    NSData *data = [line dataUsingEncoding:NSUTF8StringEncoding];
    if (![[NSFileManager defaultManager] fileExistsAtPath:path]) [[NSFileManager defaultManager] createFileAtPath:path contents:nil attributes:@{NSFilePosixPermissions: @0600}];
    NSFileHandle *file = [NSFileHandle fileHandleForWritingAtPath:path];
    [file seekToEndOfFile];
    [file writeData:data];
    [file closeFile];
}

static void OpenProject(NSString *cwd, NSString *terminal) {
    BOOL isDirectory = NO;
    if (cwd.length == 0 || ![[NSFileManager defaultManager] fileExistsAtPath:cwd isDirectory:&isDirectory] || !isDirectory) {
        DebugLog([NSString stringWithFormat:@"notification click cwd invalid; fallback to home (%@)", cwd]);
        cwd = NSHomeDirectory();
    }
    NSString *script;
    if ([terminal isEqualToString:@"iterm2"]) {
        script = [NSString stringWithFormat:@"tell application \"iTerm\"\n activate\n if (count of windows) = 0 then create window with default profile\n tell current session of current window to write text %@\nend tell", AS([NSString stringWithFormat:@"cd -- %@", ShellQuote(cwd)])];
    } else {
        script = [NSString stringWithFormat:@"tell application \"Terminal\"\n activate\n do script %@\nend tell", AS([NSString stringWithFormat:@"cd -- %@", ShellQuote(cwd)])];
    }
    NSAppleScript *appleScript = [[NSAppleScript alloc] initWithSource:script];
    NSDictionary *error = nil;
    [appleScript executeAndReturnError:&error];
    if (error) fprintf(stderr, "AgentBell: open terminal failed: %s\n", [[error description] UTF8String]);
}

@implementation BellDelegate
- (void)userNotificationCenter:(UNUserNotificationCenter *)center didReceiveNotificationResponse:(UNNotificationResponse *)response withCompletionHandler:(void (^)(void))completionHandler {
    NSDictionary *info = response.notification.request.content.userInfo;
    DebugLog([NSString stringWithFormat:@"notification click source=%@ type=%@ session=%@ cwd=%@", info[@"source"] ?: @"", info[@"type"] ?: @"", info[@"session_id"] ?: @"", info[@"cwd"] ?: @""]);
    OpenProject(info[@"cwd"], info[@"terminal"] ?: @"terminal");
    self.handled = YES;
    completionHandler();
}
- (void)userNotificationCenter:(UNUserNotificationCenter *)center willPresentNotification:(UNNotification *)notification withCompletionHandler:(void (^)(UNNotificationPresentationOptions))completionHandler {
    completionHandler(UNNotificationPresentationOptionBanner | UNNotificationPresentationOptionList);
}
@end

int main(int argc, const char *argv[]) {
    @autoreleasepool {
        if (argc == 2 && strcmp(argv[1], "--check") == 0) {
            dispatch_semaphore_t checked = dispatch_semaphore_create(0);
            __block UNAuthorizationStatus status = UNAuthorizationStatusNotDetermined;
            [[UNUserNotificationCenter currentNotificationCenter] getNotificationSettingsWithCompletionHandler:^(UNNotificationSettings *settings) {
                status = settings.authorizationStatus;
                dispatch_semaphore_signal(checked);
            }];
            dispatch_semaphore_wait(checked, dispatch_time(DISPATCH_TIME_NOW, 3 * NSEC_PER_SEC));
            const char *label = "unknown";
            if (status == UNAuthorizationStatusAuthorized || status == UNAuthorizationStatusProvisional) label = "authorized";
            else if (status == UNAuthorizationStatusDenied) label = "denied";
            else if (status == UNAuthorizationStatusNotDetermined) label = "notDetermined";
            puts(label);
            return 0;
        }
        UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
        BellDelegate *delegate = [BellDelegate new];
        center.delegate = delegate;
        if (argc < 6) {
            // Notification Center launches the app bundle on click. The payload
            // is delivered to this delegate; this short-lived response process
            // does not remain resident after handling it.
            [[NSApplication sharedApplication] setActivationPolicy:NSApplicationActivationPolicyAccessory];
            NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:30.0];
            while (!delegate.handled && deadline.timeIntervalSinceNow > 0) {
                [[NSRunLoop currentRunLoop] runMode:NSDefaultRunLoopMode beforeDate:[NSDate dateWithTimeIntervalSinceNow:0.25]];
            }
            return delegate.handled ? 0 : 1;
        }
        NSString *title = [NSString stringWithUTF8String:argv[1]] ?: @"AgentBell";
        NSString *subtitle = [NSString stringWithUTF8String:argv[2]] ?: @"";
        NSString *message = [NSString stringWithUTF8String:argv[3]] ?: @"";
        NSString *cwd = [NSString stringWithUTF8String:argv[4]] ?: @"";
        NSString *terminal = [NSString stringWithUTF8String:argv[5]] ?: @"terminal";
        NSString *source = argc > 6 ? [NSString stringWithUTF8String:argv[6]] : @"";
        NSString *type = argc > 7 ? [NSString stringWithUTF8String:argv[7]] : @"";
        NSString *session = argc > 8 ? [NSString stringWithUTF8String:argv[8]] : @"";
        [center requestAuthorizationWithOptions:UNAuthorizationOptionAlert completionHandler:^(BOOL granted, NSError *error) {
            if (!granted || error) fprintf(stderr, "AgentBell: notification permission unavailable\n");
        }];
        UNMutableNotificationContent *content = [UNMutableNotificationContent new];
        content.title = title;
        content.subtitle = subtitle;
        content.body = message;
        content.userInfo = @{ @"cwd": cwd, @"terminal": terminal, @"source": source, @"type": type, @"session_id": session };
        NSString *identifier = [NSString stringWithFormat:@"agentbell-%@", NSUUID.UUID.UUIDString];
        UNNotificationRequest *request = [UNNotificationRequest requestWithIdentifier:identifier content:content trigger:nil];
        dispatch_semaphore_t posted = dispatch_semaphore_create(0);
        __block BOOL failed = NO;
        [center addNotificationRequest:request withCompletionHandler:^(NSError *error) {
            if (error) {
                failed = YES;
                fprintf(stderr, "AgentBell: notification failed: %s\n", error.localizedDescription.UTF8String);
            }
            dispatch_semaphore_signal(posted);
        }];
        long waitResult = dispatch_semaphore_wait(posted, dispatch_time(DISPATCH_TIME_NOW, 5 * NSEC_PER_SEC));
        return (waitResult == 0 && !failed) ? 0 : 1;
    }
}
