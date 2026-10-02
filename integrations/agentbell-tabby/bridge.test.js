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
    } finally { bridge.close() }
})
