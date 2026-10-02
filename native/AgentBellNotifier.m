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
        script = [NSString stringWithFormat:@"tell application \"iTerm\"\n activate\n if (count of windows) = 0 then\n  set targetWindow to (create window with default profile)\n  set targetSession to current session of targetWindow\n else\n  tell current window to set targetTab to (create tab with default profile)\n  set targetSession to current session of targetTab\n end if\n tell targetSession to write text %@\nend tell", AS([NSString stringWithFormat:@"cd -- %@", ShellQuote(cwd)])];
    } else {
        BOOL wasRunning = NO;
        for (NSRunningApplication *runningApp in NSWorkspace.sharedWorkspace.runningApplications) {
            if ([runningApp.bundleIdentifier isEqualToString:@"com.apple.Terminal"]) {
                wasRunning = YES;
                break;
            }
        }
        // A cold launch creates a blank window automatically. Reuse that
        // window instead of creating a second one, but leave existing sessions
        // alone when Terminal was already running.
        script = [NSString stringWithFormat:@"tell application \"Terminal\"\n set reuseWindow to %@\n if (count of windows) = 0 then set reuseWindow to true\n activate\n if reuseWindow and (count of windows) > 0 then\n  do script %@ in front window\n else\n  do script %@\n end if\nend tell", wasRunning ? @"false" : @"true", AS([NSString stringWithFormat:@"cd -- %@", ShellQuote(cwd)]), AS([NSString stringWithFormat:@"cd -- %@", ShellQuote(cwd)])];
    }
    NSAppleScript *appleScript = [[NSAppleScript alloc] initWithSource:script];
    NSDictionary *error = nil;
    [appleScript executeAndReturnError:&error];
    if (error) {
        DebugLog([NSString stringWithFormat:@"notification open_project failed terminal=%@ error=%@", terminal, error]);
        fprintf(stderr, "AgentBell: open terminal failed: %s\n", [[error description] UTF8String]);
    } else {
        DebugLog([NSString stringWithFormat:@"notification open_project ok terminal=%@ cwd=%@", terminal, cwd]);
    }
}

static void StopResponseLoop(void) {
    [NSApp stop:nil];
    // stop: takes effect after an event is dispatched. Wake the event loop
    // even when this is called by a dispatch block rather than an NSEvent.
    [NSApp postEvent:[NSEvent otherEventWithType:NSEventTypeApplicationDefined
                                      location:NSZeroPoint modifierFlags:0
                                     timestamp:0 windowNumber:0 context:nil
                                       subtype:0 data1:0 data2:0] atStart:NO];
}

@implementation BellDelegate
- (void)userNotificationCenter:(UNUserNotificationCenter *)center didReceiveNotificationResponse:(UNNotificationResponse *)response withCompletionHandler:(void (^)(void))completionHandler {
    NSDictionary *info = response.notification.request.content.userInfo;
    DebugLog([NSString stringWithFormat:@"notification click source=%@ type=%@ session=%@ cwd=%@", info[@"source"] ?: @"", info[@"type"] ?: @"", info[@"session_id"] ?: @"", info[@"cwd"] ?: @""]);
    dispatch_async(dispatch_get_main_queue(), ^{
        if (![response.actionIdentifier isEqualToString:UNNotificationDismissActionIdentifier]) {
            OpenProject(info[@"cwd"], info[@"terminal"] ?: @"terminal");
        }
        self.handled = YES;
        completionHandler();
        StopResponseLoop();
    });
}
- (void)userNotificationCenter:(UNUserNotificationCenter *)center willPresentNotification:(UNNotification *)notification withCompletionHandler:(void (^)(UNNotificationPresentationOptions))completionHandler {
    completionHandler(UNNotificationPresentationOptionBanner | UNNotificationPresentationOptionList);
}
@end

int main(int argc, const char *argv[]) {
    @autoreleasepool {
        NSApplication *app = [NSApplication sharedApplication];
        [app setActivationPolicy:NSApplicationActivationPolicyAccessory];
        UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
        BellDelegate *delegate = [BellDelegate new];
        center.delegate = delegate;
        // Deliver the launch Apple Event after installing the notification
        // delegate, so a click can reach us when the posting process has exited.
        [app finishLaunching];
        if (argc == 2 && strcmp(argv[1], "--check") == 0) {
            dispatch_semaphore_t checked = dispatch_semaphore_create(0);
            __block UNAuthorizationStatus status = -1;
            [center getNotificationSettingsWithCompletionHandler:^(UNNotificationSettings *settings) {
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
        if (argc < 6) {
            // Notification Center launches the app bundle on click. The payload
            // is delivered to this delegate; this short-lived response process
            // does not remain resident after handling it.
            DebugLog(@"notification response process launched");
            dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 30 * NSEC_PER_SEC), dispatch_get_main_queue(), ^{
                if (!delegate.handled) DebugLog(@"notification response timed out");
                StopResponseLoop();
            });
            // AppKit must dispatch the launch Apple Event, not just service
            // Foundation's run loop, for a cold-start notification click.
            [app run];
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
        UNMutableNotificationContent *content = [UNMutableNotificationContent new];
        content.title = title;
        content.subtitle = subtitle;
        content.body = message;
        content.userInfo = @{ @"cwd": cwd, @"terminal": terminal, @"source": source, @"type": type, @"session_id": session };
        NSString *identifier = [NSString stringWithFormat:@"agentbell-%@", NSUUID.UUID.UUIDString];
        UNNotificationRequest *request = [UNNotificationRequest requestWithIdentifier:identifier content:content trigger:nil];
        __block BOOL finished = NO;
        __block BOOL failed = YES;
        // Posting must wait for authorization; keep the main run loop alive
        // while macOS presents its first-run permission dialog.
        [center requestAuthorizationWithOptions:UNAuthorizationOptionAlert completionHandler:^(BOOL granted, NSError *error) {
            if (!granted || error) {
                dispatch_async(dispatch_get_main_queue(), ^{
                    fprintf(stderr, "AgentBell: notification permission unavailable: %s\n", error ? error.localizedDescription.UTF8String : "denied");
                    finished = YES;
                });
                return;
            }
            [center addNotificationRequest:request withCompletionHandler:^(NSError *postError) {
                dispatch_async(dispatch_get_main_queue(), ^{
                    failed = postError != nil;
                    if (postError) fprintf(stderr, "AgentBell: notification failed: %s\n", postError.localizedDescription.UTF8String);
                    finished = YES;
                });
            }];
        }];
        NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:30.0];
        while (!finished && deadline.timeIntervalSinceNow > 0) {
            [[NSRunLoop currentRunLoop] runMode:NSDefaultRunLoopMode beforeDate:[NSDate dateWithTimeIntervalSinceNow:0.1]];
        }
        if (!finished) fprintf(stderr, "AgentBell: notification authorization or delivery timed out\n");
        return (finished && !failed) ? 0 : 1;
    }
}
