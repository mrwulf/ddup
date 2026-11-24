#!/usr/bin/env -S node --experimental-strip-types
// Captures the dashboard screenshots in ../screenshots: dashboard-dark, dashboard-light, dashboard-mobile-dark and auto-refresh-menu
//
// The page is served by ddup (any build with the dashboard embedded) and /api/status is replaced with the sample data below, which has only example.com names and documentation addresses
// Never point this at a real deployment: the images would show its names and addresses
// tab-in-browser.webp and tab-states.webp are not made here: they need a real Chrome window, which has a tab strip (see FORK.md)
//
// Usage: pnpm run build:screenshots [url]
// - url: where ddup serves the dashboard (default http://localhost:7401, as in config.sample.yaml)
// - CHROME_BIN: path to a Chrome or Chromium binary to use instead of the browser that Playwright installs
import { writeFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium, type Browser } from '@playwright/test'

const outDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', 'screenshots')
const url = process.argv[2] ?? 'http://localhost:7401'

const ago = (minutes: number): string => new Date(Date.now() - minutes * 60_000).toISOString()

const endpoint = (name: string, target: string, extra: Record<string, unknown> = {}): Record<string, unknown> => ({
    name,
    healthy: true,
    target,
    type: target.endsWith('.cfargotunnel.com') ? 'CNAME' : target.includes(':') ? 'AAAA' : 'A',
    since: ago(360),
    ...extra,
})

const failing = (failureCount: number, sinceMinutes: number): Record<string, unknown> => ({
    healthy: false,
    failureCount,
    since: ago(sinceMinutes),
    lastFailure: ago(0.5),
})

const tunnel = '0a1b2c3d.cfargotunnel.com'

// Three healthy domains, two warnings and an error, with failover tiers, CNAME and proxied targets and a dynamic DNS endpoint
const sampleStatus = (): Record<string, unknown> => ({
    'api.example.com': {
        lastUpdated: ago(1),
        provider: 'cloudflare',
        endpoints: [
            endpoint('api-1', '198.51.100.11', { priority: 0, active: true }),
            endpoint('api-2', '198.51.100.12', { priority: 0, ...failing(3, 2) }),
        ],
    },
    'app.example.com': {
        lastUpdated: ago(1),
        provider: 'cloudflare',
        endpoints: [
            endpoint('primary', '198.51.100.21', { priority: 0, ...failing(5, 4) }),
            endpoint('tunnel', tunnel, { proxied: true, priority: 1, active: true }),
        ],
    },
    'fast-lb.example.com': {
        lastUpdated: ago(1),
        provider: 'cloudflare',
        endpoints: [
            endpoint('tunnel', tunnel, { proxied: true, priority: 0, active: true }),
            endpoint('vps-eu', '192.0.2.20', { priority: 1 }),
            endpoint('vps-us', '192.0.2.10', { priority: 1 }),
        ],
    },
    'home.example.com': {
        lastUpdated: ago(3),
        provider: 'cloudflare-ddns',
        endpoints: [endpoint('home', '203.0.113.45', { active: true, since: ago(2880) })],
    },
    'ingress-lb.example.com': {
        lastUpdated: ago(1),
        provider: 'cloudflare',
        endpoints: [
            endpoint('vps-eu', '192.0.2.20', { priority: 0, active: true }),
            endpoint('vps-us', '192.0.2.10', { priority: 0, active: true }),
            endpoint('tunnel', tunnel, { proxied: true, priority: 1 }),
        ],
    },
    'status.example.com': {
        lastUpdated: ago(2),
        provider: 'cloudflare',
        error: 'failed to update records: provider returned 502',
        endpoints: [
            endpoint('status-1', '198.51.100.31', failing(12, 12)),
            endpoint('status-2', '2001:db8::31', failing(12, 12)),
        ],
    },
})

type Shot = {
    name: string
    width: number
    height: number
    theme: 'light' | 'dark'
    // Capture the whole page, not only the viewport
    fullPage: boolean
    // Open the menu of the split auto-refresh button
    refreshMenu?: boolean
}

// Sizes are in CSS pixels and are captured at 2x
const shots: Shot[] = [
    { name: 'dashboard-dark', width: 1440, height: 900, theme: 'dark', fullPage: true },
    { name: 'dashboard-light', width: 1440, height: 900, theme: 'light', fullPage: true },
    { name: 'dashboard-mobile-dark', width: 390, height: 844, theme: 'dark', fullPage: true },
    { name: 'auto-refresh-menu', width: 1440, height: 900, theme: 'light', fullPage: false, refreshMenu: true },
]

// Chromium can't write WebP screenshots, but its canvas can encode them
const toWebp = async (browser: Browser, png: Buffer): Promise<Buffer> => {
    const page = await browser.newPage()
    const base64 = await page.evaluate(async (pngBase64) => {
        const img = new Image()
        img.src = `data:image/png;base64,${pngBase64}`
        await img.decode()
        const canvas = document.createElement('canvas')
        canvas.width = img.width
        canvas.height = img.height
        canvas.getContext('2d')?.drawImage(img, 0, 0)
        const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/webp', 0.85))
        const bytes = new Uint8Array(await (blob as Blob).arrayBuffer())
        let binary = ''
        for (let i = 0; i < bytes.length; i += 0x8000) {
            binary += String.fromCharCode(...bytes.subarray(i, i + 0x8000))
        }
        return btoa(binary)
    }, png.toString('base64'))
    await page.close()
    return Buffer.from(base64, 'base64')
}

const browser = await chromium.launch({ executablePath: process.env.CHROME_BIN })
for (const shot of shots) {
    const context = await browser.newContext({
        viewport: { width: shot.width, height: shot.height },
        deviceScaleFactor: 2,
        colorScheme: shot.theme,
    })
    // The theme button shows the explicit choice, as in the committed images
    await context.addInitScript((theme) => localStorage.setItem('ddup-theme', theme), shot.theme)
    const page = await context.newPage()
    await page.route('**/api/status', (route) => route.fulfill({ json: sampleStatus() }))
    await page.goto(url)
    await page.locator('[data-slot=card-title]').first().waitFor()
    if (shot.refreshMenu) {
        await page.getByLabel('More refresh options').click()
    }
    // Lets the animations end
    await page.waitForTimeout(800)

    const png = await page.screenshot({ fullPage: shot.fullPage })
    writeFileSync(path.join(outDir, `${shot.name}.webp`), await toWebp(browser, png))
    console.log(`Wrote ${shot.name}.webp`)
    await context.close()
}
await browser.close()
