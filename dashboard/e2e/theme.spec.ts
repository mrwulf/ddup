import { expect, test, type Page } from '@playwright/test'

const domains = {
    'healthy.example': {
        lastUpdated: '2026-01-02T03:04:05Z',
        provider: 'cloudflare',
        endpoints: [{ healthy: true, ip: '192.0.2.1' }],
    },
}

const storageKey = 'ddup-theme'

const mockStatus = async (page: Page) => {
    await page.route('**/api/status', (route) => route.fulfill({ json: domains }))
}

const themeButton = (page: Page, current: 'system' | 'light' | 'dark') =>
    page.getByRole('button', { name: new RegExp(`^Theme: ${current}`) })

const isDark = (page: Page) => page.evaluate(() => document.documentElement.classList.contains('dark'))

const openPage = async (page: Page) => {
    await mockStatus(page)
    await page.goto('/')
    await expect(page.locator('[data-slot="card-title"]')).toHaveText(['healthy.example'])
}

test.describe('theme switcher', () => {
    test('cycles system, light, dark and back, and says what the next one is', async ({ page }) => {
        await openPage(page)

        await expect(themeButton(page, 'system')).toHaveAttribute('aria-label', 'Theme: system (click for light)')
        await themeButton(page, 'system').click()
        await expect(themeButton(page, 'light')).toHaveAttribute('aria-label', 'Theme: light (click for dark)')
        await themeButton(page, 'light').click()
        await expect(themeButton(page, 'dark')).toHaveAttribute('aria-label', 'Theme: dark (click for system)')
        await themeButton(page, 'dark').click()
        await expect(themeButton(page, 'system')).toBeVisible()
    })

    test('system follows the preference of the browser, also when it changes', async ({ page }) => {
        await page.emulateMedia({ colorScheme: 'dark' })
        await openPage(page)
        expect(await isDark(page)).toBe(true)

        await page.emulateMedia({ colorScheme: 'light' })
        await expect.poll(() => isDark(page)).toBe(false)
    })

    test('light and dark ignore the preference of the browser', async ({ page }) => {
        await page.emulateMedia({ colorScheme: 'dark' })
        await openPage(page)

        await themeButton(page, 'system').click()
        await expect(themeButton(page, 'light')).toBeVisible()
        expect(await isDark(page)).toBe(false)

        await themeButton(page, 'light').click()
        await expect(themeButton(page, 'dark')).toBeVisible()
        expect(await isDark(page)).toBe(true)

        // Back to the system: the browser prefers dark
        await themeButton(page, 'dark').click()
        await expect(themeButton(page, 'system')).toBeVisible()
        expect(await isDark(page)).toBe(true)
    })

    test('the choice is kept after a reload, and set before the page renders', async ({ page }) => {
        await page.emulateMedia({ colorScheme: 'light' })
        await openPage(page)
        await themeButton(page, 'system').click()
        await themeButton(page, 'light').click()
        await expect(themeButton(page, 'dark')).toBeVisible()
        expect(await page.evaluate((key) => localStorage.getItem(key), storageKey)).toBe('dark')

        // The class is already on the page when the document has been parsed, before the dashboard is drawn
        await page.reload({ waitUntil: 'commit' })
        await page.waitForFunction(() => document.documentElement.classList.contains('dark'))
        await expect(themeButton(page, 'dark')).toBeVisible()
    })

    test('going back to the system forgets the choice', async ({ page }) => {
        await openPage(page)
        await themeButton(page, 'system').click()
        await themeButton(page, 'light').click()
        await themeButton(page, 'dark').click()
        await expect(themeButton(page, 'system')).toBeVisible()
        expect(await page.evaluate((key) => localStorage.getItem(key), storageKey)).toBeNull()
    })

    test('works when the storage is not available', async ({ page }) => {
        await page.addInitScript(() => {
            const fail = () => {
                throw new DOMException('denied', 'SecurityError')
            }
            Object.defineProperty(window, 'localStorage', {
                get: fail,
            })
        })
        await openPage(page)

        await themeButton(page, 'system').click()
        await themeButton(page, 'light').click()
        await expect(themeButton(page, 'dark')).toBeVisible()
        expect(await isDark(page)).toBe(true)
    })

    test('the button says the theme in text and not only with an icon', async ({ page }) => {
        await openPage(page)
        const button = themeButton(page, 'system')
        await expect(button).toHaveAttribute('title', 'Theme: system (click for light)')
        await expect(button.locator('svg')).toHaveAttribute('aria-hidden', 'true')
    })
})
