on run argv
    set targetTTY to item 1 of argv
    if application id "com.apple.Terminal" is not running then error "Terminal is not running"
    tell application id "com.apple.Terminal"
        -- Resolve IDs now, rather than retaining an item reference to a live
        -- window collection whose order may change during notification clicks.
        set windowIDs to get id of windows
        repeat with windowID in windowIDs
            set targetWindowID to contents of windowID
            set tabList to {}
            -- Terminal includes auxiliary windows with a missing ID.
            if class of targetWindowID is integer then
                try
                    set tabList to get tabs of window id targetWindowID
                on error errorMessage number errorNumber
                    -- Closing windows may have no scriptable tabs.
                    -- Permission and other failures remain visible to Core.
                    if errorNumber is not -1728 then error errorMessage number errorNumber
                end try
            end if
            repeat with tabReference in tabList
                set targetTab to contents of tabReference
                if tty of targetTab is targetTTY then
                    set selected of targetTab to true
                    set miniaturized of window id targetWindowID to false
                    set frontmost of window id targetWindowID to true
                    activate
                    return targetWindowID as text
                end if
            end repeat
        end repeat
    end tell
    error "Terminal context expired"
end run
