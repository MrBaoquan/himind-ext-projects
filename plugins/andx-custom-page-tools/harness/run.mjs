import fs from 'node:fs'
import path from 'node:path'
import os from 'node:os'
import { spawn } from 'node:child_process'
import { createServer } from 'node:http'

const args = parseArgs(process.argv.slice(2))
const sdkRoot = path.resolve(requiredArg('sdk-root'))
const outputDir = path.resolve(requiredArg('output-dir'))
const pageRoot = args['page-root'] ? path.resolve(args['page-root']) : ''
const publicCode = args['public-code'] || 'P1-E1'
const timeoutMs = Number(args['timeout-ms'] || 20_000)
const chromePath = args.chrome || process.env.ANDX_CHROME_PATH || findChrome()
const sdkDist = resolveSdkDist(sdkRoot)
const reportPath = path.join(outputDir, 'bridge-report.json')
const screenshotPath = path.join(outputDir, 'bridge-fixture.png')
// Chrome owns profile ACLs and asynchronous cache handles on Windows. Keep its
// disposable data outside the user's candidate/evidence tree; the host owns
// temporary-root retention. Never recursively remove browser-owned directories.
const chromeProfile = fs.mkdtempSync(path.join(os.tmpdir(), 'andx-bridge-profile-'))

fs.mkdirSync(outputDir, { recursive: true })

const consoleErrors = []
const exceptions = []
const networkFailures = []
const responses = new Map()
let server
let chrome
let cdp
let ws

const report = {
  schema: 'andx-custom-page-bridge-test/v1',
  startedAt: new Date().toISOString(),
  source: pageRoot ? 'fixture-injected-page' : 'embedded-fixture',
  pageBusinessVerified: false,
  pageRoot,
  sdkRoot,
  sdkDist,
  chromePath,
  assertions: {},
  consoleErrors,
  exceptions,
  networkFailures,
  httpResponses: [],
}

async function main() {
  let deadline
  try {
    await Promise.race([
      runFixture(),
      new Promise((_, reject) => { deadline = setTimeout(() => reject(new Error('Bridge test deadline exceeded')), timeoutMs) }),
    ])
  } catch (error) {
    report.ok = false
    report.error = error instanceof Error ? error.message : String(error)
    report.completedAt = new Date().toISOString()
  } finally {
    clearTimeout(deadline)
    await stopChrome()
    try { cdp?.dispose() } catch {}
    try { ws?.close() } catch {}
    try { server?.closeAllConnections(); await new Promise((resolve) => server ? server.close(resolve) : resolve()) } catch {}
    fs.writeFileSync(reportPath, `${JSON.stringify(report, null, 2)}\n`)
    // Failed CDP/fetch work may still own timers. Exit only after the report is flushed.
    process.stdout.write(`${JSON.stringify(report, null, 2)}\n`, () => process.exit(report.ok ? 0 : 1))
  }
}

async function stopChrome() {
  if (!chrome?.pid || chrome.exitCode !== null || chrome.signalCode !== null) return
  // Ask the browser to flush and close its profile before forcing the tree down.
  if (cdp) {
    await Promise.race([
      cdp.send('Browser.close').catch(() => {}),
      new Promise(resolve => setTimeout(resolve, 1000)),
    ])
    if (chrome.exitCode === null && chrome.signalCode === null) {
      await new Promise(resolve => {
        const timer = setTimeout(resolve, 1000)
        chrome.once('exit', () => { clearTimeout(timer); resolve() })
      })
    }
    if (chrome.exitCode !== null || chrome.signalCode !== null) return
  }
  if (process.platform === 'win32') {
    const killer = spawn('taskkill.exe', ['/PID', String(chrome.pid), '/T', '/F'], { stdio: 'ignore', windowsHide: true })
    await new Promise(resolve => {
      const timer = setTimeout(() => { killer.kill(); resolve() }, 3000)
      const done = () => { clearTimeout(timer); resolve() }
      killer.once('error', done)
      killer.once('exit', done)
    })
  } else {
    chrome.kill()
  }
  if (chrome.exitCode === null && chrome.signalCode === null) {
    await new Promise(resolve => {
      const timer = setTimeout(resolve, 1000)
      chrome.once('exit', () => { clearTimeout(timer); resolve() })
    })
  }
}

async function runFixture() {
  const port = await startServer()
  const entryUrl = `http://127.0.0.1:${port}/x/${encodeURIComponent(publicCode)}`
  report.entryUrl = entryUrl
  chrome = spawn(chromePath, [
    '--headless=new',
    '--disable-gpu',
    '--disable-dev-shm-usage',
    '--disable-extensions',
    '--disable-background-networking',
    '--hide-scrollbars',
    '--no-first-run',
    '--no-default-browser-check',
    '--remote-allow-origins=*',
    '--remote-debugging-port=0',
    `--user-data-dir=${chromeProfile}`,
    '--window-size=430,900',
    'about:blank',
  ], { stdio: 'ignore', windowsHide: true })
  await new Promise((resolve, reject) => {
    chrome.once('spawn', resolve)
    chrome.once('error', reject)
  })

  const activePortFile = path.join(chromeProfile, 'DevToolsActivePort')
  await waitFile(activePortFile, timeoutMs)
  const chromePort = Number(fs.readFileSync(activePortFile, 'utf8').split(/\r?\n/)[0])
  if (!Number.isInteger(chromePort) || chromePort <= 0) throw new Error('invalid Chrome DevTools port')

  const opened = await openPage(chromePort, entryUrl, timeoutMs)
  cdp = opened.cdp
  ws = opened.ws
  await waitValue(cdp, 'window.__ANDX_TEST_RESULT__ !== undefined', timeoutMs)
  const evaluated = await cdp.send('Runtime.evaluate', {
    expression: 'window.__ANDX_TEST_RESULT__',
    returnByValue: true,
    awaitPromise: true,
  })
  const fixtureResult = evaluated.result?.value
  if (!fixtureResult || typeof fixtureResult !== 'object') throw new Error('fixture did not return a result object')

  const screenshot = await cdp.send('Page.captureScreenshot', {
    format: 'png',
    captureBeyondViewport: false,
    fromSurface: true,
  })
  fs.writeFileSync(screenshotPath, Buffer.from(screenshot.data, 'base64'))

  report.assertions.fixtureResult = fixtureResult.ok === true
  report.assertions.handshake = fixtureResult.handshake === true
  report.assertions.capabilityNegotiation = fixtureResult.capabilityNegotiation === true
  report.assertions.action = fixtureResult.action === true
  report.assertions.status = fixtureResult.status === true
  report.assertions.paymentSuccess = fixtureResult.paymentSuccess === true
  report.assertions.paymentCancelled = fixtureResult.paymentCancelled === true
  report.assertions.stateEvent = fixtureResult.stateEvent === true
  report.assertions.paymentEvent = fixtureResult.paymentEvent === true
  report.assertions.revoked = fixtureResult.revoked === true
  report.assertions.consoleClean = consoleErrors.length === 0 && exceptions.length === 0
  report.ok = Object.values(report.assertions).every(Boolean)
  report.fixture = fixtureResult
  report.httpResponses = [...responses.entries()].map(([url, status]) => ({ url, status }))
  report.completedAt = new Date().toISOString()
  report.reportPath = reportPath
  report.screenshotPath = screenshotPath
}

function parseArgs(argv) {
  const result = {}
  for (let index = 0; index < argv.length; index += 1) {
    const key = argv[index]
    if (!key.startsWith('--')) continue
    result[key.slice(2)] = argv[index + 1]
    index += 1
  }
  return result
}

function requiredArg(name) {
  const value = args[name]
  if (!value) throw new Error(`missing required argument --${name}`)
  return value
}

function resolveSdkDist(root) {
  const candidates = [path.join(root, 'dist'), root]
  for (const candidate of candidates) {
    if (fs.existsSync(path.join(candidate, 'index.js'))) return candidate
  }
  throw new Error(`@mrbaoquan/andx dist/index.js not found under ${root}`)
}

function findChrome() {
  const candidates = [
    'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
    'C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe',
    'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe',
  ]
  const found = candidates.find((candidate) => fs.existsSync(candidate))
  if (!found) throw new Error('Chrome or Edge executable not found; pass --chrome')
  return found
}

function contentType(file) {
  return ({
    '.html': 'text/html; charset=utf-8',
    '.js': 'text/javascript; charset=utf-8',
    '.mjs': 'text/javascript; charset=utf-8',
    '.css': 'text/css; charset=utf-8',
    '.json': 'application/json; charset=utf-8',
    '.png': 'image/png',
    '.jpg': 'image/jpeg',
    '.jpeg': 'image/jpeg',
    '.svg': 'image/svg+xml',
  })[path.extname(file).toLowerCase()] || 'application/octet-stream'
}

function serveFile(response, file) {
  if (!fs.existsSync(file) || !fs.statSync(file).isFile()) {
    response.writeHead(404, corsHeaders({ 'content-type': 'text/plain; charset=utf-8' }))
    response.end('not found')
    return
  }
  response.writeHead(200, corsHeaders({ 'content-type': contentType(file), 'cache-control': 'no-store' }))
  fs.createReadStream(file).pipe(response)
}

function corsHeaders(headers = {}) {
  return { ...headers, 'access-control-allow-origin': '*', 'cross-origin-resource-policy': 'cross-origin' }
}

function safeJoin(root, relative) {
  const full = path.resolve(root, relative)
  return full === root || full.startsWith(root + path.sep) ? full : null
}

async function startServer() {
  server = createServer((request, response) => {
    const url = new URL(request.url || '/', 'http://127.0.0.1')
    responses.set(url.pathname, 200)
    if (url.pathname === '/favicon.ico') {
      response.writeHead(204, corsHeaders())
      response.end()
      return
    }
    if (request.method === 'OPTIONS') {
      response.writeHead(204, corsHeaders())
      response.end()
      return
    }
    if (url.pathname === '/' || url.pathname === '/index.html' || url.pathname.startsWith('/x/')) {
      response.writeHead(200, corsHeaders({ 'content-type': 'text/html; charset=utf-8', 'cache-control': 'no-store' }))
      response.end(hostHtml())
      return
    }
    if (url.pathname === '/host.js') {
      response.writeHead(200, corsHeaders({ 'content-type': 'text/javascript; charset=utf-8', 'cache-control': 'no-store' }))
      response.end(hostScript())
      return
    }
    if (url.pathname === '/__andx_test__/bridge.js') {
      response.writeHead(200, corsHeaders({ 'content-type': 'text/javascript; charset=utf-8', 'cache-control': 'no-store' }))
      response.end(fixtureScript())
      return
    }
    if (url.pathname === '/page/' || url.pathname === '/page/index.html') {
      response.writeHead(200, corsHeaders({ 'content-type': 'text/html; charset=utf-8', 'cache-control': 'no-store' }))
      response.end(pageRoot ? pageIndexHtml(pageRoot) : fixtureHtml())
      return
    }
    if (pageRoot && url.pathname.startsWith('/page/')) {
      const relative = decodeURIComponent(url.pathname.slice('/page/'.length))
      const file = safeJoin(pageRoot, relative)
      if (file) serveFile(response, file)
      else response.writeHead(403)
      return
    }
    if (!pageRoot && url.pathname === '/page/app.js') {
      response.writeHead(200, corsHeaders({ 'content-type': 'text/javascript; charset=utf-8', 'cache-control': 'no-store' }))
      response.end(fixtureScript())
      return
    }
    if (url.pathname.startsWith('/sdk/')) {
      const relative = decodeURIComponent(url.pathname.slice('/sdk/'.length))
      const file = safeJoin(sdkDist, relative)
      if (file) serveFile(response, file)
      else {
        response.writeHead(403)
        response.end('forbidden')
      }
      return
    }
    response.writeHead(404, corsHeaders({ 'content-type': 'text/plain; charset=utf-8' }))
    response.end('not found')
  })
  await new Promise((resolve, reject) => {
    server.once('error', reject)
    server.listen(0, '127.0.0.1', resolve)
  })
  return server.address().port
}

function pageIndexHtml(root) {
  const indexPath = path.join(root, 'index.html')
  if (!fs.existsSync(indexPath)) throw new Error(`page-root index.html not found: ${indexPath}`)
  const html = fs.readFileSync(indexPath, 'utf8')
  return html.replace('</body>', '<script type="module" src="/__andx_test__/bridge.js"></script></body>')
}

function hostHtml() {
  return `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>AndX Bridge Fixture</title>
<style>html,body{margin:0;width:100%;height:100%;background:#10151f;color:#fff;font:14px sans-serif}iframe{width:430px;height:900px;border:0;display:block}#result{position:fixed;right:8px;bottom:8px;background:#1d2736;padding:4px 8px;border-radius:4px}</style>
</head><body><iframe id="frame" sandbox="allow-scripts"></iframe><div id="result">running</div><script type="module" src="/host.js"></script></body></html>`
}

function hostScript() {
  return `import { createInteractionHost } from '/sdk/index.js'

const expiresAt = new Date(Date.now() + 10 * 60_000).toISOString()
const session = {
  sessionNo: 'session-harness',
  publicCode: 'P1-E1',
  exhibitId: 'exhibit-1',
  exhibitCode: 'E1',
  exhibitName: 'Bridge Fixture',
  channel: 'MINI_WEBVIEW',
  status: 'ACTIVE',
  amount: 500,
  currency: 'CNY',
  expiresAt,
}
const payment = { enabled: true, amount: 500, currency: 'CNY', status: 'IDLE', mode: 'MINI_PROGRAM' }
const idleStatus = { revision: 1, state: 'IDLE', idle: true, orderNo: null, commandId: null, observedAt: new Date().toISOString() }
const frame = document.getElementById('frame')
let host
const resultNode = document.getElementById('result')

window.addEventListener('message', (event) => {
  if (event.source !== frame.contentWindow) return
  if (!event.data || event.data.type !== 'andx.fixture.result') return
  window.__ANDX_TEST_RESULT__ = event.data.result
  resultNode.textContent = event.data.result.ok ? 'passed' : 'failed'
})

host = createInteractionHost({
  window,
  frame,
  session,
  payment,
  expectedFrameOrigin: null,
  adapters: {
    async getSession() { return session },
    async getStatus() { return idleStatus },
    async requestAction({ actionKey, args }) { return { actionKey, accepted: true, echo: args } },
    async requestPayment({ idempotencyKey }) {
      if (idempotencyKey.includes('cancel')) {
        const snapshot = { intentNo: 'intent-cancel', status: 'CANCELLED', amount: 500, currency: 'CNY', orderNo: null, expiresAt, mode: 'MINI_PROGRAM' }
        setTimeout(() => host.revoke('PARENT_CLOSED'), 150)
        return snapshot
      }
      const snapshot = { intentNo: 'intent-success', status: 'PAID', amount: 500, currency: 'CNY', orderNo: 'ORDER-1', expiresAt, mode: 'MINI_PROGRAM' }
      host.publishPayment(snapshot)
      setTimeout(() => host.publishState({ revision: 2, state: 'PLAYING', idle: false, orderNo: 'ORDER-1', commandId: 'CMD-1', observedAt: new Date().toISOString() }), 50)
      return snapshot
    },
  },
})

frame.src = '/page/?parentOrigin=' + encodeURIComponent(location.origin)
host.start()
`
}

function fixtureHtml() {
  return `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Bridge Fixture Page</title></head><body><main id="status">starting</main><script type="module" src="./app.js"></script></body></html>`
}

function fixtureScript() {
  return `import { createCustomPageBridge } from '/sdk/index.js'

const statusNode = document.getElementById('status')
const parentOrigin = new URLSearchParams(location.search).get('parentOrigin')
let finished = false
const stateEvents = []
const paymentEvents = []

function finish(result) {
  if (finished) return
  finished = true
  statusNode.textContent = result.ok ? 'Bridge 验证通过' : 'Bridge 验证失败'
  const detailNode = document.getElementById('detail')
  if (detailNode) {
    detailNode.textContent = result.ok
      ? '握手、能力协商、状态读取、支付与撤销均通过'
      : (result.error || '请检查测试报告')
  }
  window.parent.postMessage({ type: 'andx.fixture.result', result }, parentOrigin)
}

async function main() {
  if (!parentOrigin) throw new Error('parentOrigin missing')
  const bridge = createCustomPageBridge({
    window,
    parentWindow: window.parent,
    parentOrigin,
    client: { name: 'andx-page-bridge-fixture', version: '1.0.0' },
    requiredCapabilities: ['andx.payment.request', 'andx.action.request'],
    optionalCapabilities: ['andx.status.read', 'andx.payment.events', 'andx.lifecycle.events'],
  })
  bridge.onState((snapshot) => stateEvents.push(snapshot))
  bridge.onPayment((snapshot) => paymentEvents.push(snapshot))
  const revoked = new Promise((resolve) => bridge.onRevoked(resolve))
  bridge.start()
  const ready = await bridge.whenReady()
  const status = await bridge.getStatus()
  const action = await bridge.requestAction('welcome.submit', { text: 'fixture' }, { idempotencyKey: 'fixture-action' })
  const success = await bridge.requestPayment({ idempotencyKey: 'fixture-success' })
  const cancelled = await bridge.requestPayment({ idempotencyKey: 'fixture-cancel' })
  const revokeEvent = await Promise.race([
    revoked,
    new Promise((_, reject) => setTimeout(() => reject(new Error('revoke timeout')), 4000)),
  ])
  const capabilityNegotiation = ['andx.payment.request', 'andx.status.read', 'andx.payment.events', 'andx.lifecycle.events']
    .every((capability) => ready.grantedCapabilities.includes(capability))
  const result = {
    ok: true,
    handshake: typeof ready.bridgeSessionId === 'string' && ready.bridgeSessionId.length > 0,
    capabilityNegotiation,
    status: status.state === 'IDLE' && status.idle === true,
    action: action.accepted === true && action.actionKey === 'welcome.submit',
    paymentSuccess: success.status === 'PAID' && success.orderNo === 'ORDER-1',
    paymentCancelled: cancelled.status === 'CANCELLED',
    stateEvent: stateEvents.some((snapshot) => snapshot.state === 'PLAYING' && snapshot.idle === false),
    paymentEvent: paymentEvents.some((snapshot) => snapshot.status === 'PAID'),
    revoked: revokeEvent.reason === 'PARENT_CLOSED',
    ready: {
      protocolVersion: ready.protocolVersion,
      grantedCapabilities: ready.grantedCapabilities,
      sessionNo: ready.session.sessionNo,
    },
    stateEvents,
    paymentEvents,
    revokeEvent,
  }
  finish(result)
}

main().catch((error) => finish({ ok: false, error: error instanceof Error ? error.message : String(error) }))
`
}

class CdpClient {
  constructor(socket) {
    this.socket = socket
    this.nextId = 1
    this.pending = new Map()
    socket.addEventListener('message', (event) => {
      let message
      try { message = JSON.parse(String(event.data)) } catch { return }
      if (message.id) {
        const pending = this.pending.get(message.id)
        if (!pending) return
        this.pending.delete(message.id)
        if (message.error) pending.reject(new Error(message.error.message || 'CDP error'))
        else pending.resolve(message.result)
        return
      }
      if (message.method === 'Runtime.consoleAPICalled' && message.params?.type === 'error') {
        consoleErrors.push((message.params.args || []).map((arg) => arg.value ?? arg.description ?? '').join(' '))
      }
      if (message.method === 'Runtime.exceptionThrown') {
        exceptions.push(message.params?.exceptionDetails?.text || 'runtime exception')
      }
      if (message.method === 'Log.entryAdded' && message.params?.entry?.level === 'error') {
        consoleErrors.push(message.params.entry.text || 'log error')
      }
      if (message.method === 'Network.loadingFailed' && !message.params?.canceled) {
        networkFailures.push(message.params?.errorText || 'network failure')
      }
    })
  }

  send(method, params = {}) {
    const id = this.nextId++
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => { this.pending.delete(id); reject(new Error('CDP command timed out: ' + method)) }, Math.min(timeoutMs, 15000))
      this.pending.set(id, { resolve: (value) => { clearTimeout(timer); resolve(value) }, reject: (error) => { clearTimeout(timer); reject(error) } })
      this.socket.send(JSON.stringify({ id, method, params }))
    })
  }

  dispose() {
    for (const pending of this.pending.values()) pending.reject(new Error('Bridge test closed'))
    this.pending.clear()
    this.socket.close()
  }
}

async function openPage(port, url, timeout) {
  const response = await fetch(`http://127.0.0.1:${port}/json/new?${encodeURIComponent('about:blank')}`, { method: 'PUT' })
  if (!response.ok) throw new Error(`cannot create Chrome target: ${response.status}`)
  const target = await response.json()
  const socket = new WebSocket(target.webSocketDebuggerUrl)
  ws = socket
  await new Promise((resolve, reject) => {
    socket.addEventListener('open', resolve, { once: true })
    socket.addEventListener('error', reject, { once: true })
  })
  const client = new CdpClient(socket)
  cdp = client
  await client.send('Page.enable')
  await client.send('Runtime.enable')
  await client.send('Log.enable')
  await client.send('Network.enable')
  await client.send('Network.setUserAgentOverride', {
    userAgent: 'Mozilla/5.0 (Linux; Android 13) AppleWebKit/537.36 MicroMessenger/8.0.50 Mobile Safari/537.36',
  })
  await client.send('Emulation.setDeviceMetricsOverride', { width: 430, height: 900, deviceScaleFactor: 1, mobile: true })
  await client.send('Page.navigate', { url })
  await waitValue(client, 'document.readyState === "interactive" || document.readyState === "complete"', timeout)
  return { cdp: client, ws: socket }
}

async function waitFile(file, timeout) {
  const started = Date.now()
  while (Date.now() - started < timeout) {
    if (fs.existsSync(file)) return
    await new Promise((resolve) => setTimeout(resolve, 50))
  }
  throw new Error(`timeout waiting for ${file}`)
}

async function waitValue(client, expression, timeout) {
  const started = Date.now()
  while (Date.now() - started < timeout) {
    const response = await client.send('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true })
    if (response.result?.value) return response.result.value
    await new Promise((resolve) => setTimeout(resolve, 100))
  }
  throw new Error(`timeout waiting for expression: ${expression}`)
}

await main()
