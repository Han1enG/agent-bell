#import <AppKit/AppKit.h>
#import <Carbon/Carbon.h>
#import <UserNotifications/UserNotifications.h>
#import <string.h>

@interface BellDelegate : NSObject <UNUserNotificationCenterDelegate>
@property(nonatomic, assign) BOOL handled;
@end

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

static void ReturnToContext(NSDictionary *info) {
    // Resolve our packaged binary locally; never execute a path from payload.
    NSString *binary = [NSBundle.mainBundle.bundlePath stringByAppendingPathComponent:@"Contents/MacOS/agentbell"];
    if (![[NSFileManager defaultManager] isExecutableFileAtPath:binary]) {
        binary = [[[NSProcessInfo processInfo].arguments.firstObject stringByDeletingLastPathComponent] stringByAppendingPathComponent:@"agentbell"];
    }
    NSString *target = info[@"return_target"];
    if (![target isKindOfClass:NSString.class] || [target isEqualToString:@"null"]) return;
    NSTask *task = [NSTask new];
    task.executableURL = [NSURL fileURLWithPath:binary];
    task.arguments = @[@"return", target];
    NSError *error = nil;
    if (![task launchAndReturnError:&error]) DebugLog([NSString stringWithFormat:@"return launch failed: %@", error]);
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
    DebugLog([NSString stringWithFormat:@"notification click source=%@ type=%@ session=%@ ", info[@"source"] ?: @"", info[@"type"] ?: @"", info[@"session_id"] ?: @""]);
    dispatch_async(dispatch_get_main_queue(), ^{
        if (![response.actionIdentifier isEqualToString:UNNotificationDismissActionIdentifier]) {
            ReturnToContext(info);
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
        if (argc == 2 && strcmp(argv[1], "--check-terminal-automation") == 0) {
            const char *bundle = "com.apple.Terminal";
            AEAddressDesc target = {typeNull, NULL};
            OSStatus status = AECreateDesc(typeApplicationBundleID, bundle, strlen(bundle), &target);
            if (status == noErr) {
                status = AEDeterminePermissionToAutomateTarget(&target, kCoreEventClass, kAEGetData, false);
                AEDisposeDesc(&target);
            }
            const char *label = "unverified";
            if (status == noErr) label = "authorized";
            else if (status == errAEEventNotPermitted) label = "denied";
            else if (status == errAEEventWouldRequireUserConsent) label = "not_requested";
            else if (status == procNotFound) label = "app_not_running";
            puts(label);
            return 0;
        }
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
        NSString *target = [NSString stringWithUTF8String:argv[4]] ?: @"null";
        NSString *actionTitle = [NSString stringWithUTF8String:argv[5]] ?: @"";
        NSString *source = argc > 7 ? [NSString stringWithUTF8String:argv[7]] : @"";
        NSString *type = argc > 8 ? [NSString stringWithUTF8String:argv[8]] : @"";
        NSString *session = argc > 9 ? [NSString stringWithUTF8String:argv[9]] : @"";
        UNMutableNotificationContent *content = [UNMutableNotificationContent new];
        content.title = title;
        content.subtitle = subtitle;
        content.body = message;
        content.userInfo = @{ @"return_target": target, @"source": source, @"type": type, @"session_id": session };
        if (actionTitle.length > 0) {
            // Per-title categories allow multiple originating applications in the list.
            NSString *categoryID = [@"RETURN_TO_CONTEXT_" stringByAppendingString:actionTitle];
            UNNotificationAction *action = [UNNotificationAction actionWithIdentifier:@"RETURN_TO_CONTEXT" title:actionTitle options:UNNotificationActionOptionForeground];
            UNNotificationCategory *category = [UNNotificationCategory categoryWithIdentifier:categoryID actions:@[action] intentIdentifiers:@[] options:UNNotificationCategoryOptionNone];
            dispatch_semaphore_t registered = dispatch_semaphore_create(0);
            [center getNotificationCategoriesWithCompletionHandler:^(NSSet<UNNotificationCategory *> *categories) {
                NSMutableSet *updated = [categories mutableCopy];
                [updated addObject:category];
                [center setNotificationCategories:updated];
                dispatch_semaphore_signal(registered);
            }];
            dispatch_semaphore_wait(registered, dispatch_time(DISPATCH_TIME_NOW, 3 * NSEC_PER_SEC));
            content.categoryIdentifier = categoryID;
        }
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
