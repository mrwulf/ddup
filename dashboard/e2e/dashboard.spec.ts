import { expect, test, type Locator, type Page, type Request } from '@playwright/test'

// Same shape as the response of the /api/status endpoint
interface DomainStatusEndpoint {
    name?: string
    healthy: boolean
    ip: string
    type?: string
    proxied?: boolean
    priority?: number
    active?: boolean
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

const lastUpdated = '2026-01-02T03:04:05Z'

const searchPlaceholder = 'Search domains, endpoints or IP addresses...'

// One domain for each state the dashboard can show
// Healthy: healthy.example, ipv6.example
// Warning: partial.example
// Unhealthy: down.example, dns-error.example, no-endpoints.example
const domains: DomainsResponse = {
    'healthy.example': {
        lastUpdated,
        provider: 'cloudflare',
        endpoints: [
            { healthy: true, ip: '192.0.2.1' },
            { healthy: true, ip: '192.0.2.2', failureCount: 1 },
        ],
    },
    'partial.example': {
        lastUpdated,
        provider: 'ovh',
        endpoints: [
            { healthy: true, ip: '198.51.100.1' },
            { healthy: false, ip: '198.51.100.2', failureCount: 3 },
        ],
    },
    'down.example': {
        lastUpdated,
        provider: 'azure',
        endpoints: [{ healthy: false, ip: '203.0.113.1', failureCount: 5 }],
    },
    'dns-error.example': {
        lastUpdated,
        provider: 'cloudflare',
        error: 'Error updating DNS records: boom',
        endpoints: [{ healthy: true, ip: '203.0.113.9' }],
    },
    'no-endpoints.example': {
        lastUpdated,
        provider: 'cloudflare',
        endpoints: [],
    },
    'ipv6.example': {
        lastUpdated,
        provider: 'ovh',
        endpoints: [{ healthy: true, ip: '2001:db8::1' }],
    },
}
const domainNames = Object.keys(domains)

// Same as domains, but down.example has recovered
const recoveredDomains: DomainsResponse = {
    ...domains,
    'down.example': {
        lastUpdated,
        provider: 'azure',
        endpoints: [{ healthy: true, ip: '203.0.113.1' }],
    },
}

// Mocks the /api/status endpoint
// Change body to change the next responses
const mockStatus = async (page: Page, body: DomainsResponse) => {
    const mock = { body, requests: 0 }
    await page.route('**/api/status', (route) => {
        mock.requests++
        return route.fulfill({ json: mock.body })
    })
    return mock
}

// Returns a promise that the test resolves by calling release, to control when a mocked response is sent
const deferred = () => {
    let release!: () => void
    const promise = new Promise<void>((resolve) => {
        release = resolve
    })
    return { promise, release }
}

const buildInfo = { version: '1.2.3', buildId: '1.2.3', commit: '0123456789abcdef', buildDate: '2026-01-01T00:00:00Z' }

// The footer loads the version of the build once, and is left out if that fails
test.beforeEach(async ({ page }) => {
    await page.route('**/api/info', (route) => route.fulfill({ json: buildInfo }))
})

const cardTitles = (page: Page) => page.locator('[data-slot="card-title"]')

const domainCard = (page: Page, name: string) =>
    page.locator('[data-slot="card"]').filter({ has: page.getByText(name, { exact: true }) })

const domainBadge = (page: Page, name: string) =>
    domainCard(page, name).locator('[data-slot="card-header"] [data-slot="badge"]')

// The row of an endpoint is the closest bordered box around its address
const endpointRow = (card: Locator, address: string) =>
    card.getByText(address, { exact: true }).locator('xpath=ancestor::div[contains(@class, "rounded-lg")][1]')

// The count is the first span of a segment in the summary bar
const summaryCount = (
    page: Page,
    title: 'Healthy domains' | 'Domains with some unhealthy endpoints' | 'Unhealthy domains'
) => page.getByTitle(title, { exact: true }).locator('span').first()

const expectSummary = async (page: Page, healthy: number, warning: number, unhealthy: number) => {
    await expect(summaryCount(page, 'Healthy domains')).toHaveText(String(healthy))
    await expect(summaryCount(page, 'Domains with some unhealthy endpoints')).toHaveText(String(warning))
    await expect(summaryCount(page, 'Unhealthy domains')).toHaveText(String(unhealthy))
}

test.describe('domain list', () => {
    test.beforeEach(async ({ page }) => {
        await mockStatus(page, domains)
        await page.goto('/')
        await expect(cardTitles(page)).toHaveText(domainNames)
    })

    test('shows the health of each domain', async ({ page }) => {
        await expect(domainBadge(page, 'healthy.example')).toHaveText('Healthy')
        await expect(domainBadge(page, 'ipv6.example')).toHaveText('Healthy')
        await expect(domainBadge(page, 'partial.example')).toHaveText('Warning')
        await expect(domainBadge(page, 'down.example')).toHaveText('Unhealthy')
        // A DNS error makes the domain unhealthy, even if its endpoints are healthy
        await expect(domainBadge(page, 'dns-error.example')).toHaveText('Unhealthy')
        await expect(domainBadge(page, 'no-endpoints.example')).toHaveText('Unhealthy')

        await expect(page.getByText('Last updated:')).toBeVisible()
    })

    test('shows the details of each domain', async ({ page }) => {
        const card = domainCard(page, 'partial.example')
        await expect(card).toContainText('Provider: ovh')
        await expect(card).toContainText('1/2 endpoints healthy')
        await expect(card).toContainText('Endpoints (2)')

        // The state and the failures are shown as icons with a text for screen readers
        const healthyRow = endpointRow(card, '198.51.100.1')
        await expect(healthyRow).toContainText('Healthy')
        await expect(healthyRow).not.toContainText('consecutive failed')
        const unhealthyRow = endpointRow(card, '198.51.100.2')
        await expect(unhealthyRow).toContainText('Unhealthy')
        await expect(unhealthyRow).toContainText('3 consecutive failed checks')

        // Endpoints that are still healthy can have recent failures
        await expect(endpointRow(domainCard(page, 'healthy.example'), '192.0.2.2')).toContainText(
            '1 consecutive failed check'
        )

        // Without different priorities, the priority and the standby state are not shown
        await expect(card.getByTitle(/^Priority/)).toHaveCount(0)
        await expect(card.getByTitle('Healthy, on standby')).toHaveCount(0)

        await expect(domainCard(page, 'dns-error.example')).toContainText('Domain Error')
        await expect(domainCard(page, 'dns-error.example')).toContainText('Error updating DNS records: boom')

        const emptyCard = domainCard(page, 'no-endpoints.example')
        await expect(emptyCard).toContainText('0/0 endpoints healthy')
        await expect(emptyCard).not.toContainText('Endpoints (')
    })

    test('counts the domains by health', async ({ page }) => {
        await expectSummary(page, 2, 1, 3)
    })

    test('search filters by domain name', async ({ page }) => {
        const search = page.getByPlaceholder(searchPlaceholder)

        await search.fill('PARTIAL')
        await expect(cardTitles(page)).toHaveText(['partial.example'])
        await expectSummary(page, 0, 1, 0)

        await search.fill('')
        await expect(cardTitles(page)).toHaveText(domainNames)
        await expectSummary(page, 2, 1, 3)
    })

    test('search filters by endpoint IP', async ({ page }) => {
        const search = page.getByPlaceholder(searchPlaceholder)

        await search.fill('2001:DB8')
        await expect(cardTitles(page)).toHaveText(['ipv6.example'])

        // Unhealthy endpoints match too
        await search.fill('198.51.100.2')
        await expect(cardTitles(page)).toHaveText(['partial.example'])
    })

    test('search with no results shows a message', async ({ page }) => {
        await page.getByPlaceholder(searchPlaceholder).fill('nomatch')
        await expect(cardTitles(page)).toHaveCount(0)
        await expect(page.getByText('No domains or endpoints found matching "nomatch"')).toBeVisible()
        await expectSummary(page, 0, 0, 0)
    })
})

test.describe('loading the status', () => {
    test('shows a loading indicator until the status arrives', async ({ page }) => {
        const response = deferred()
        await page.route('**/api/status', async (route) => {
            await response.promise
            await route.fulfill({ json: domains })
        })
        await page.goto('/')

        const loading = page.getByText('Loading domain data...')
        await expect(loading).toBeVisible()
        await expect(cardTitles(page)).toHaveCount(0)

        response.release()
        await expect(loading).toBeHidden()
        await expect(cardTitles(page)).toHaveText(domainNames)
    })

    test('shows a message when no domains are configured', async ({ page }) => {
        await mockStatus(page, {})
        await page.goto('/')

        await expect(page.getByText('No domains configured or available')).toBeVisible()
        await expect(page.getByTitle('Healthy domains', { exact: true })).toBeHidden()
    })

    test('shows an error when the status cannot be loaded', async ({ page }) => {
        await page.route('**/api/status', (route) => route.fulfill({ status: 500, body: 'boom' }))
        await page.goto('/')

        await expect(page.getByText(/^Failed to fetch domain data: HTTP error: 500/)).toBeVisible()
        await expect(cardTitles(page)).toHaveCount(0)
        await expect(page.getByTitle('Healthy domains', { exact: true })).toBeHidden()
        await expect(page.getByText('No domains configured or available')).toBeHidden()
    })
})

test.describe('refreshing', () => {
    test('auto-refresh reloads the status every minute until it is turned off', async ({ page }) => {
        await page.clock.install()
        const status = await mockStatus(page, domains)
        await page.goto('/')
        await expect(cardTitles(page)).toHaveText(domainNames)
        expect(status.requests).toBe(1)

        status.body = recoveredDomains
        await page.clock.fastForward('01:00')
        await expect.poll(() => status.requests).toBe(2)
        await expect(domainBadge(page, 'down.example')).toHaveText('Healthy')

        await page.getByRole('button', { name: 'Auto-refresh ON' }).click()
        const turnOn = page.getByRole('button', { name: 'Auto-refresh OFF' })
        await expect(turnOn).toBeVisible()
        await page.clock.fastForward('05:00')

        // Turning it back on loads the status right away
        // If the interval had kept running while off, there would be more requests
        await turnOn.click()
        await expect(page.getByRole('button', { name: 'Auto-refresh ON' })).toBeVisible()
        await expect.poll(() => status.requests).toBe(3)
        await expect(page.getByText('Loading domain data...')).toBeHidden()
        expect(status.requests).toBe(3)
    })

    test('Refresh now reloads the status', async ({ page }) => {
        const status = await mockStatus(page, domains)
        await page.goto('/')
        await expect(domainBadge(page, 'down.example')).toHaveText('Unhealthy')

        status.body = recoveredDomains
        await page.getByRole('button', { name: 'More refresh options' }).click()
        await page.getByRole('menuitem', { name: 'Refresh now' }).click()

        await expect(domainBadge(page, 'down.example')).toHaveText('Healthy')
        expect(status.requests).toBe(2)
    })
})

test.describe('Check now', () => {
    test('runs the health checks and shows the new status', async ({ page }) => {
        await mockStatus(page, domains)
        const response = deferred()
        const checkRequests: Request[] = []
        await page.route('**/api/check', async (route) => {
            checkRequests.push(route.request())
            await response.promise
            await route.fulfill({ json: recoveredDomains })
        })
        await page.goto('/')
        await expect(domainBadge(page, 'down.example')).toHaveText('Unhealthy')

        const checkNow = page.getByRole('button', { name: 'Check now' })
        await checkNow.click()

        // The button is disabled until the check completes
        await expect(page.getByRole('button', { name: 'Checking…' })).toBeDisabled()

        response.release()
        await expect(checkNow).toBeEnabled()
        await expect(domainBadge(page, 'down.example')).toHaveText('Healthy')

        // The server requires this header, which blocks cross-site requests
        expect(checkRequests).toHaveLength(1)
        expect(checkRequests[0].method()).toBe('POST')
        expect(checkRequests[0].headers()['x-requested-by']).toBe('ddup-dashboard')
    })

    test('shows an error when the check fails', async ({ page }) => {
        await mockStatus(page, domains)
        await page.route('**/api/check', (route) =>
            route.fulfill({
                status: 504,
                json: { code: 'api_check_timeout', message: 'The check did not complete in time' },
            })
        )
        await page.goto('/')
        await expect(cardTitles(page)).toHaveText(domainNames)

        const checkNow = page.getByRole('button', { name: 'Check now' })
        await checkNow.click()

        await expect(page.getByText(/^Failed to run check: HTTP error: 504/)).toBeVisible()
        await expect(checkNow).toBeEnabled()
    })
})

// Endpoints of a domain with a failover: priorities, a CNAME target behind Cloudflare's proxy and a name for each
const tieredDomains: DomainsResponse = {
    'tiered.example': {
        lastUpdated,
        provider: 'cloudflare',
        endpoints: [
            { name: 'tunnel', healthy: true, ip: 'tunnel.example.com', type: 'CNAME', proxied: true, priority: 1 },
            { name: 'us', healthy: true, ip: '192.0.2.10', type: 'A', priority: 0, active: true },
            { name: 'eu', healthy: false, ip: '2001:db8::10', type: 'AAAA', priority: 0, failureCount: 4 },
        ],
    },
}

test.describe('endpoints', () => {
    test.beforeEach(async ({ page }) => {
        await mockStatus(page, tieredDomains)
        await page.goto('/')
        await expect(cardTitles(page)).toHaveText(['tiered.example'])
    })

    test('lists the endpoints by priority, with their name and address', async ({ page }) => {
        const card = domainCard(page, 'tiered.example')
        await expect(card).toContainText('Endpoints (3)')
        await expect(card).toContainText('2/3 endpoints healthy')

        // The published endpoint comes first within its priority, and the name is the main label
        const names = card.locator('div.truncate.text-sm')
        await expect(names).toHaveText(['us', 'eu', 'tunnel'])
        await expect(endpointRow(card, '192.0.2.10')).toContainText('us')
        await expect(endpointRow(card, '2001:db8::10')).toContainText('eu')
        await expect(endpointRow(card, 'tunnel.example.com')).toContainText('tunnel')
    })

    test('shows the type, the priority and whether the endpoint is published', async ({ page }) => {
        const card = domainCard(page, 'tiered.example')

        const published = endpointRow(card, '192.0.2.10')
        await expect(published.getByTitle('Priority 0')).toBeVisible()
        await expect(published.getByTitle('Published in DNS')).toBeVisible()
        await expect(published.getByTitle('CNAME record')).toHaveCount(0)

        const failing = endpointRow(card, '2001:db8::10')
        await expect(failing.getByTitle('Priority 0')).toBeVisible()
        await expect(failing.getByTitle('4 consecutive failed checks')).toBeVisible()
        // Only healthy endpoints can be published or on standby
        await expect(failing.getByTitle('Published in DNS')).toHaveCount(0)
        await expect(failing.getByTitle('Healthy, on standby')).toHaveCount(0)

        const standby = endpointRow(card, 'tunnel.example.com')
        await expect(standby.getByTitle('Priority 1')).toBeVisible()
        await expect(standby.getByTitle('Healthy, on standby')).toBeVisible()
        await expect(standby.getByTitle('CNAME record')).toBeVisible()
        await expect(standby.getByTitle('Proxied by Cloudflare')).toBeVisible()
    })

    test('search matches the name of an endpoint', async ({ page }) => {
        const search = page.getByPlaceholder(searchPlaceholder)

        await search.fill('TUNNEL')
        await expect(cardTitles(page)).toHaveText(['tiered.example'])

        await search.fill('nomatch')
        await expect(cardTitles(page)).toHaveCount(0)
    })
})

test.describe('endpoint state', () => {
    test('says since when an endpoint is healthy or failing', async ({ page }) => {
        await mockStatus(page, {
            'since.example': {
                lastUpdated,
                provider: 'cloudflare',
                endpoints: [
                    {
                        healthy: true,
                        ip: '192.0.2.1',
                        since: '2026-01-02T03:00:00Z',
                        lastFailure: '2026-01-02T02:00:00Z',
                    },
                    { healthy: true, ip: '192.0.2.2', since: '2026-01-02T03:00:00Z', sinceStart: true },
                    { healthy: false, ip: '192.0.2.3', since: '2026-01-02T03:00:00Z', failureCount: 2 },
                ],
            },
        })
        await page.goto('/')
        const card = domainCard(page, 'since.example')
        await expect(card).toContainText('Endpoints (3)')

        await expect(endpointRow(card, '192.0.2.1')).toContainText(/Healthy since .+ \(last failure .+\)/)
        await expect(endpointRow(card, '192.0.2.2')).toContainText(/Healthy since ddup started \(.+\)/)
        const failing = endpointRow(card, '192.0.2.3')
        await expect(failing).toContainText(/Failing since .+/)
        await expect(failing).not.toContainText('last failure')
    })
})

test.describe('tab icon', () => {
    const iconHref = (page: Page) => page.locator('link[rel~="icon"]').first().getAttribute('href')

    const healthyOnly: DomainsResponse = { 'healthy.example': domains['healthy.example'] }
    const warningOnly: DomainsResponse = { ...healthyOnly, 'partial.example': domains['partial.example'] }

    test('shows the worst status of all the domains', async ({ page }) => {
        const status = await mockStatus(page, healthyOnly)
        await page.goto('/')
        await expect(cardTitles(page)).toHaveCount(1)
        await expect.poll(() => iconHref(page)).toMatch(/\/healthy-[\w-]+\.png$/)

        status.body = warningOnly
        await page.getByRole('button', { name: 'More refresh options' }).click()
        await page.getByRole('menuitem', { name: 'Refresh now' }).click()
        await expect.poll(() => iconHref(page)).toMatch(/\/warning-[\w-]+\.png$/)

        status.body = domains
        await page.getByRole('button', { name: 'More refresh options' }).click()
        await page.getByRole('menuitem', { name: 'Refresh now' }).click()
        await expect.poll(() => iconHref(page)).toMatch(/\/unhealthy-[\w-]+\.png$/)
    })

    test('shows an unhealthy status when the status cannot be loaded', async ({ page }) => {
        await page.route('**/api/status', (route) => route.fulfill({ status: 500, body: 'boom' }))
        await page.goto('/')
        await expect(page.getByText(/^Failed to fetch domain data/)).toBeVisible()
        await expect.poll(() => iconHref(page)).toMatch(/\/unhealthy-[\w-]+\.png$/)
    })
})

test.describe('footer', () => {
    test('shows the version of the build', async ({ page }) => {
        await mockStatus(page, domains)
        await page.goto('/')

        const footer = page.getByTitle('Version of this build of ddup')
        await expect(footer).toContainText('ddup 1.2.3')
        await expect(footer).toContainText('0123456')
    })

    test('is left out if the version cannot be loaded', async ({ page }) => {
        await page.route('**/api/info', (route) => route.fulfill({ status: 500, body: 'boom' }))
        await mockStatus(page, domains)
        await page.goto('/')
        await expect(cardTitles(page)).toHaveText(domainNames)

        await expect(page.getByTitle('Version of this build of ddup')).toHaveCount(0)
    })
})

test('the theme button cycles through the themes', async ({ page }) => {
    await mockStatus(page, domains)
    await page.goto('/')
    await expect(cardTitles(page)).toHaveText(domainNames)

    await page.getByRole('button', { name: /^Theme: system/ }).click()
    await page.getByRole('button', { name: /^Theme: light/ }).click()
    await page.getByRole('button', { name: /^Theme: dark/ }).click()
    await expect(page.getByRole('button', { name: /^Theme: system/ })).toBeVisible()
})

test('enabled buttons show a pointer cursor', async ({ page }) => {
    await mockStatus(page, domains)
    await page.goto('/')
    await expect(cardTitles(page)).toHaveText(domainNames)

    // Open the menu so its items are included
    await page.getByRole('button', { name: 'More refresh options' }).click()
    await expect(page.getByRole('menuitem', { name: 'Refresh now' })).toBeVisible()

    const cursors = await page
        .locator('button:enabled, [role="button"]:not([aria-disabled="true"]), [role="menuitem"]:not([data-disabled])')
        .evaluateAll((elements) =>
            elements.map((el) => ({
                element: el.getAttribute('aria-label') || el.textContent?.trim(),
                cursor: getComputedStyle(el).cursor,
            }))
        )
    expect(cursors.length).toBeGreaterThan(0)
    expect(cursors).toEqual(cursors.map((c) => ({ ...c, cursor: 'pointer' })))
})
