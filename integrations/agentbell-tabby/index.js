'use strict'
const { NgModule, Inject, NgZone } = require('@angular/core')
const { AppService, HostWindowService } = require('tabby-core')
const { TerminalService } = require('tabby-local')
const { startBridge } = require('./bridge')
const { prepareContext } = require('./context')
class AgentBellModule {
    constructor(app, window, zone, terminal) {
        try {
            this.bridge = startBridge(app, window, zone, undefined, async sessionID => {
                const helper = require('path').join(require('os').homedir(), 'Applications', 'AgentBell.app', 'Contents', 'MacOS', 'agentbell')
                const info = require('fs').statSync(helper)
                if (!info.isFile() || !(info.mode & 0o111) || (info.mode & 0o022)) throw new Error('unsafe launcher')
                await terminal.openTab({ type: 'local', name: 'Resume', options: { command: helper, args: ['resume-launch', sessionID] } }, null, false)
                window.bringToFront()
            })
            this.subscriptions = []
            const prepare = tab => prepareContext(tab, this.bridge.getContextID(tab))
            const watch = root => {
                const children = typeof root.getAllTabs === 'function' ? root.getAllTabs() : []
                for (const tab of children.length ? children : [root]) prepare(tab)
                if (root.tabAdded$) this.subscriptions.push(root.tabAdded$.subscribe(prepare))
            }
            app.tabs.forEach(watch)
            this.subscriptions.push(app.tabOpened$.subscribe(watch))
        }
        catch (error) { console.warn('AgentBell:', error.message) }
    }
    ngOnDestroy() {
        for (const subscription of this.subscriptions || []) subscription.unsubscribe()
        if (this.bridge) this.bridge.close()
    }
}
Inject(AppService)(AgentBellModule, undefined, 0)
Inject(HostWindowService)(AgentBellModule, undefined, 1)
Inject(NgZone)(AgentBellModule, undefined, 2)
Inject(TerminalService)(AgentBellModule, undefined, 3)
NgModule({})(AgentBellModule)
exports.default = AgentBellModule
