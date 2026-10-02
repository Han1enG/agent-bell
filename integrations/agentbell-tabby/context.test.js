const { test } = require('node:test')
const assert = require('node:assert/strict')
const { prepareContext } = require('./context')
test('new local tabs receive independent environments without modifying shared profiles', () => {
    const profile = { type: 'local', options: { env: { KEEP: 'value' } } }
    const a = { profile }, b = { profile }
    assert.equal(prepareContext(a, 'A'), true)
    assert.equal(prepareContext(b, 'B'), true)
    assert.equal(a.profile.options.env.AGENTBELL_CONTEXT_ID, 'A')
    assert.equal(b.profile.options.env.AGENTBELL_CONTEXT_ID, 'B')
    assert.equal(a.profile.options.env.KEEP, 'value')
    assert.deepEqual(profile.options.env, { KEEP: 'value' })
})
test('existing PTYs, running sessions and SSH tabs are not changed', () => {
    for (const tab of [
        { profile: { type: 'ssh', options: {} } },
        { profile: { type: 'local', options: { restoreFromPTYID: 'pty' } } },
        { profile: { type: 'local', options: {} }, session: {} },
    ]) {
        assert.equal(prepareContext(tab, 'new-id'), false)
        assert.equal(tab.profile.options.env, undefined)
    }
})
