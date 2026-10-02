const { test } = require('node:test')
const assert = require('node:assert/strict')
const net = require('net')
const fs = require('fs')
const os = require('os')
const path = require('path')
const { startBridge } = require('./bridge')
function request(socketPath, payload) {
    return new Promise((resolve, reject) => {
        const socket = net.connect(socketPath, () => socket.write(JSON.stringify(payload) + '\n'))
        let data = ''
        socket.on('data', chunk => { data += chunk })
        socket.on('end', () => resolve(JSON.parse(data)))
        socket.on('error', reject)
    })
}
test('three tabs retain distinct IDs, focus selects B, expired context fails', async () => {
    const directory = fs.mkdtempSync('/private/tmp/ab-' )
    fs.chmodSync(directory, 0o700)
    const app = { tabs: [{ title: 'same cwd' }, { title: 'coordinator' }, { title: 'same cwd' }], selectTab(tab) { this.activeTab = tab } }
    let activated = false
    const bridge = startBridge(app, { bringToFront() { activated = true } }, { run(fn) { fn() } }, directory)
    try {
        await new Promise(resolve => setImmediate(resolve))
        const list = await request(bridge.socketPath, { operation: 'list' })
        assert.equal(new Set(list.contexts.map(x => x.ContextID)).size, 3)
        assert.deepEqual(await request(bridge.socketPath, { operation: 'list' }), list)
        assert.equal((await request(bridge.socketPath, { operation: 'focus', context: list.contexts[1].ContextID })).ok, true)
        assert.equal(app.activeTab, app.tabs[1]); assert.equal(activated, true)
        app.tabs.splice(1, 1)
        assert.equal((await request(bridge.socketPath, { operation: 'focus', context: list.contexts[1].ContextID })).ok, false)
    } finally { await bridge.close(); fs.rmSync(directory, { recursive: true }) }
})

async function fixture(app) {
    const directory = fs.mkdtempSync('/private/tmp/ab-')
    fs.chmodSync(directory, 0o700)
    let activations = 0
    const bridge = startBridge(app, { bringToFront() { activations++ } }, { run(fn) { fn() } }, directory)
    await new Promise(resolve => setImmediate(resolve))
    return { bridge, activations: () => activations, close: async () => { await bridge.close(); fs.rmSync(directory, { recursive: true }) } }
}
test('100 contexts stay unique across rename, close and window restart; list does not focus', async () => {
    const tabs = Array.from({ length: 100 }, () => ({ title: 'same cwd' }))
    const app = { tabs, selectTab(tab) { this.activeTab = tab } }
    const first = await fixture(app)
    const second = await fixture(app)
    try {
        const ids = (await request(first.bridge.socketPath, { operation: 'list' })).contexts.map(c => c.ContextID)
        const restarted = (await request(second.bridge.socketPath, { operation: 'list' })).contexts.map(c => c.ContextID)
        assert.equal(new Set([...ids, ...restarted]).size, 200)
        assert.equal(first.activations(), 0)
        tabs[72].title = 'renamed'
        assert.equal((await request(first.bridge.socketPath, { operation: 'focus', context: ids[72] })).ok, true)
        assert.equal(app.activeTab, tabs[72])
        assert.equal((await request(second.bridge.socketPath, { operation: 'focus', context: ids[72] })).ok, false)
        tabs.splice(72, 1)
        assert.equal((await request(first.bridge.socketPath, { operation: 'focus', context: ids[72] })).ok, false)
        assert.equal(first.activations(), 1)
    } finally { await first.close(); await second.close() }
})
test('split pane focuses the matching child; unsupported pane focus fails before selecting any tab', async () => {
    for (const supported of [true, false]) {
        const panes = [{ title: 'same cwd' }, { title: 'same cwd' }]
        let focused, selected
        const root = { getAllTabs: () => panes }
        if (supported) root.focus = tab => { focused = tab }
        const app = { tabs: [root], selectTab(tab) { selected = tab } }
        const f = await fixture(app)
        try {
            const list = await request(f.bridge.socketPath, { operation: 'list' })
            const result = await request(f.bridge.socketPath, { operation: 'focus', context: list.contexts[1].ContextID })
            assert.equal(result.ok, supported)
            assert.equal(list.contexts[1].CanFocus, supported)
            assert.equal(focused, supported ? panes[1] : undefined)
            assert.equal(selected, supported ? root : undefined)
            assert.equal(f.activations(), supported ? 1 : 0)
        } finally { await f.close() }
    }
})
