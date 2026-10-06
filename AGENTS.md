# Local development delivery

The user requests that functional changes be compiled and deployed to their local AgentBell installation without another reminder. A change is not delivered merely because source edits or tests are complete.

- Build the signed macOS RC bundle after code or native UI changes, verify its signature and version, and update both the local CLI and AgentBell.app.
- Back up the existing App, hook configuration and SQLite state before replacement. Preserve unrelated hooks, user settings and session records.
- Restart only AgentBell, then verify that the running App and CLI report the expected version. Run health checks with access to the real App socket and process/notification APIs; sandbox failures are not installation evidence.
- Keep this work on a release candidate. Local installation is authorized; publishing a Release, creating a tag or updating the public Homebrew tap requires separate user authorization.
- Do not terminate agents or restart Tabby/IDEs to deploy AgentBell. Surface bridges can be checked without closing the user's work.
- Documentation-only changes do not require rebuilding unchanged executables.
