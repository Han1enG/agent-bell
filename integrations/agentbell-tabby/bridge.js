'use strict'
const net = require('net')
const fs = require('fs')
const path = require('path')
const os = require('os')
const { randomUUID } = require('crypto')

// Ephemeral, per-window bridge. It never manages agents or persists sessions.
function startBridge(app, hostWindow, zone, directory = path.join(os.homedir(), '.cache', 'agentbell', 'tabby'), resume) {
    fs.mkdirSync(directory, { recursive: true, mode: 0o700 })
    const stat = fs.lstatSync(directory)
    if (!stat.isDirectory() || stat.isSymbolicLink() || stat.uid !== process.getuid() || (stat.mode & 0o077)) {
        throw new Error('AgentBell bridge directory must be owned by you with mode 0700')
    }
    const windowID = randomUUID()
    const socketPath = path.join(directory, windowID + '.sock')
    const ids = new WeakMap()
    const getContextID = tab => {
        if (!ids.has(tab)) ids.set(tab, windowID + ':' + randomUUID())
        return ids.get(tab)
    }
    const contexts = () => app.tabs.flatMap(root => {
        const children = typeof root.getAllTabs === 'function' ? root.getAllTabs() : []
        const tabs = children.length ? children : [root]
        return tabs.map(tab => ({ ContextID: getContextID(tab), Title: tab.title || root.title || '', CanFocus: root === tab || typeof root.focus === 'function', tab, root }))
    })
    const server = net.createServer(socket => {
        socket.setTimeout(2000, () => socket.destroy())
        let data = ''
        socket.on('error', () => {})
        socket.on('data', async chunk => {
            data += chunk
            if (data.length > 8192) return socket.destroy()
            if (!data.includes('\n')) return
            socket.pause()
            try {
                const request = JSON.parse(data.slice(0, data.indexOf('\n')))
                if (request.operation === 'capabilities') {
                    socket.end(JSON.stringify({ resume: typeof resume === 'function' }) + '\n')
                } else if (request.operation === 'resume') {
                    if (typeof resume !== 'function') return socket.end('{"ok":false,"reason":"unsupported_surface"}\n')
                    if (typeof request.sessionID !== 'string' || !/^(claude|codex):[a-zA-Z0-9:_-]{1,1024}$/.test(request.sessionID)) return socket.end('{"ok":false,"reason":"invalid_target"}\n')
                    await zone.run(() => resume(request.sessionID))
                    socket.end('{"ok":true}\n')
                } else if (request.operation === 'status') {
                    const root = app.activeTab
                    const focused = root && typeof root.getFocusedTab === 'function' ? root.getFocusedTab() : root
                    const active = contexts().find(item => item.tab === focused)
                    socket.end(JSON.stringify({ activeContextID: active ? active.ContextID : null }) + '\n')
                } else if (request.operation === 'list') {
                    const items = await Promise.all(contexts().map(async ({ ContextID, Title, CanFocus, tab }) => {
                        const pty = tab.session && tab.session.pty
                        let ShellPID = null
                        try { if (pty && typeof pty.getPID === 'function') ShellPID = await pty.getPID() } catch (_) {}
                        return { ContextID, Title, CanFocus, ShellPID }
                    }))
                    const response = JSON.stringify({ contexts: items }) + '\n'
                    socket.end(Buffer.byteLength(response) <= 65536 ? response : '{"ok":false}\n')
                } else if (request.operation === 'focus') {
                    const target = contexts().find(item => item.ContextID === request.context)
                    if (!target) return socket.end('{"ok":false,"reason":"context_not_found"}\n')
                    if (!target.CanFocus) return socket.end('{"ok":false,"reason":"unsupported_surface"}\n')
                    zone.run(() => {
                        app.selectTab(target.root)
                        if (target.root !== target.tab && typeof target.root.focus === 'function') target.root.focus(target.tab)
                        hostWindow.bringToFront()
                    })
                    socket.end('{"ok":true}\n')
                } else socket.end('{"ok":false}\n')
            } catch (_) { socket.end('{"ok":false}\n') }
        })
    })
    server.on('error', error => console.warn('AgentBell bridge unavailable:', error.message))
    server.listen(socketPath, () => fs.chmodSync(socketPath, 0o600))
    return { socketPath, getContextID, close: () => new Promise(resolve => server.close(() => { try { fs.unlinkSync(socketPath) } catch (_) {}; resolve() })) }
}
module.exports = { startBridge }
