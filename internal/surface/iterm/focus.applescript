on run argv
    set sessionID to item 1 of argv
    set targetTTY to item 2 of argv
    if application id "com.googlecode.iterm2" is not running then error "app_not_running"
    tell application id "com.googlecode.iterm2"
        repeat with w in windows
            repeat with t in tabs of w
                repeat with s in sessions of t
                    if unique id of s is sessionID and tty of s is targetTTY then
                        set resultID to (id of w as text) & ":" & (index of t as text)
                        if item 3 of argv is "probe" then return resultID
                        activate
                        select w
                        select t
                        select s
                        return resultID
                    end if
                end repeat
            end repeat
        end repeat
    end tell
    error "context_not_found"
end run
