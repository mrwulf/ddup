import { expect, test, type Page } from '@playwright/test'

// Tests for the features of the open pull requests together: the build info (footer, header, reload),
// the endpoint history, and the endpoints of the status API with a CNAME or a looked-up address

interface DomainStatusEndpoint {
    healthy: boolean
    ip: string
    type?: string
    proxied?: boolean
    failureCount?: number
    since?: string
    sinceStart?: boolean
    lastFailure?: string
}

interface DomainStatus {
    lastUpdated: string
    provider: string
    error?: string
    endpoints: DomainStatusEndpoint[]
}

type DomainsResponse = Record<string, DomainStatus>

const pageBuild = {
    build: '1.2.3|build-42|0123456789abcdef',
    version: '1.2.3',
    buildId: 'build-42',
    commit: '0123456789abcdef',
}
const newerBuild = '1.2.4|build-43|fedcba9876543210'
const buildHeader = 'X-Ddup-Build'

// A domain with a fixed IP, a looked-up address and a proxied CNAME, as the server returns it
const mixedDomains: DomainsResponse = {
    'app.example': {
        lastUpdated: '2026-10-13T09:30:00Z',
        provider: 'cloudflare',
        endpoints: [
            {
                healthy: false,
                ip: '192.0.2.10',
                type: 'A',
                failureCount: 2,
                since: '2026-10-13T09:00:00Z',
                lastFailure: '2026-10-13T09:29:00Z',
            },
            { healthy: true, ip: '203.0.113.5', type: 'A', since: '2026-10-13T07:45:00Z', sinceStart: true },
            {
                healthy: true,
                ip: '0a1b2c3d.cfargotunnel.com',
                type: 'CNAME',
                proxied: true,
                since: '2026-10-09T08:15:00Z',
                lastFailure: '2026-10-09T08:10:00Z',
            },
        ],
    },
}

// Serves the page like the server does, with the build info in a JSON script element before </head>
const servePage = async (page: Page) => {
    const served = { documents: 0 }
    await page.route(
        (url) => url.pathname === '/',
        async (route) => {
            served.documents++
            const response = await route.fetch()
            const body = (await response.text()).replace(
                '</head>',
                `<script type="application/json" id="ddup-build">${JSON.stringify(pageBuild)}</script></head>`
            )
            await route.fulfill({ response, body })
        }
    )
    return served
}

// Mocks the API like the server does: the build in the header of every response, also the status of one domain and the check
const mockAPI = async (page: Page, body: DomainsResponse, checkBuild = pageBuild.build) => {
    const requests = { status: 0, check: 0 }
    await page.route('**/api/status', (route) => {
        requests.status++
        return route.fulfill({ json: body, headers: { [buildHeader]: pageBuild.build } })
    })
    await page.route('**/api/check', (route) => {
        requests.check++
        return route.fulfill({ json: body, headers: { [buildHeader]: checkBuild } })
    })
    return requests
}

const historyButton = (page: Page, domain: string) => page.getByRole('button', { name: `History of ${domain}` })
const historyDialog = (page: Page, domain: string) =>
    page.getByRole('dialog', { name: `Endpoint history of ${domain}` })

test.describe('mixed domain with the build info', () => {
    test.use({ locale: 'en-US', timezoneId: 'UTC' })

    test.beforeEach(async ({ page }) => {
        await page.clock.setFixedTime(new Date('2026-10-13T10:00:00Z'))
    })

    test('the history lists the IP, the looked-up address and the CNAME, and the footer is shown', async ({ page }) => {
        const served = await servePage(page)
        await mockAPI(page, mixedDomains)
        await page.goto('/')

        await expect(page.locator('footer')).toHaveText('ddup 1.2.3 · 0123456')

        await historyButton(page, 'app.example').click()
        const items = historyDialog(page, 'app.example').getByRole('listitem')
        await expect(items).toHaveCount(3)

        await expect(items.nth(0)).toContainText('192.0.2.10')
        await expect(items.nth(0)).toContainText('Unhealthy since Oct 13, 9:00 AM')
        await expect(items.nth(0)).not.toContainText('Last failure')

        await expect(items.nth(1)).toContainText('203.0.113.5')
        await expect(items.nth(1)).toContainText('Healthy since ddup started (Oct 13, 7:45 AM)')

        await expect(items.nth(2)).toContainText('0a1b2c3d.cfargotunnel.com')
        await expect(items.nth(2)).toContainText('Healthy since Oct 9, 8:15 AM')
        await expect(items.nth(2)).toContainText('Last failure: Oct 9, 8:10 AM')

        // Opening the history doesn't reload the page
        expect(served.documents).toBe(1)
    })

    test('the page is not reloaded when every API response has the build of the page', async ({ page }) => {
        const served = await servePage(page)
        const requests = await mockAPI(page, mixedDomains)
        await page.goto('/')
        await expect(page.locator('footer')).toHaveText('ddup 1.2.3 · 0123456')

        // Refresh the status, then run a check
        await page.getByRole('button', { name: 'More refresh options' }).click()
        await page.getByRole('menuitem', { name: 'Refresh now' }).click()
        await expect.poll(() => requests.status).toBe(2)

        await page.getByRole('button', { name: 'Check now' }).click()
        await expect.poll(() => requests.check).toBe(1)
        await expect(page.getByRole('button', { name: 'Check now' })).toBeEnabled()

        expect(served.documents).toBe(1)
        await expect(page.locator('footer')).toHaveText('ddup 1.2.3 · 0123456')
    })

    test('the response of a check with a newer build reloads the page', async ({ page }) => {
        const served = await servePage(page)
        await mockAPI(page, mixedDomains, newerBuild)
        await page.goto('/')
        await expect(historyButton(page, 'app.example')).toBeVisible()

        await page.getByRole('button', { name: 'Check now' }).click()
        await expect.poll(() => served.documents).toBe(2)
        await expect(historyButton(page, 'app.example')).toBeVisible()
    })
})
