'use strict'

// AppService.tabOpened$ is emitted before Angular initializes the tab view.
// Clone per-tab profile options, so no shared profile/global environment changes.
function prepareContext(tab, contextID) {
    if (!tab.profile || tab.profile.type !== 'local' || tab.session) return false
    const options = tab.profile.options || {}
    // A recovered PTY is already running: its environment cannot be changed.
    if (options.restoreFromPTYID) return false
    tab.profile = { ...tab.profile, options: { ...options, env: {
        ...options.env,
        AGENTBELL_SURFACE: 'tabby',
        AGENTBELL_CONTEXT_ID: contextID,
    } } }
    tab.sessionOptions = tab.profile.options
    return true
}
module.exports = { prepareContext }
