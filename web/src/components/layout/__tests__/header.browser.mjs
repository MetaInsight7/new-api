import assert from 'node:assert/strict'
import fs from 'node:fs/promises'
import { createRequire } from 'node:module'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const loadInstalledLibrary = createRequire(import.meta.url)
const { chromium, expect } = loadInstalledLibrary('playwright/test')

const baseUrl = process.env.CONSOLE_TEST_URL || 'http://127.0.0.1:5173'
const outputDirectory =
  process.env.CONSOLE_TEST_OUTPUT_DIR || '/tmp/tokendazi-header-verification'
const websiteOrigin = (
  process.env.CONSOLE_EXPECTED_WEBSITE_ORIGIN || 'https://tokendazi.com'
).replace(/\/+$/, '')
const documentationUrl =
  process.env.CONSOLE_EXPECTED_DOCUMENTATION_URL ||
  'https://docs.tokendazi.com/'
const username = process.env.CONSOLE_TEST_USERNAME
const password = process.env.CONSOLE_TEST_PASSWORD
const webRoot = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  '../../../..'
)

const interfaceLanguages = [
  ['en', 'en', 'English'],
  ['zhCN', 'zh', '简体中文'],
  ['zhTW', 'zh-TW', '繁體中文'],
  ['fr', 'fr', 'Français'],
  ['ru', 'ru', 'Русский'],
  ['ja', 'ja', '日本語'],
  ['vi', 'vi', 'Tiếng Việt'],
]

async function main() {
  assert.ok(
    username && password,
    'Set CONSOLE_TEST_USERNAME and CONSOLE_TEST_PASSWORD.'
  )
  await fs.mkdir(outputDirectory, { recursive: true })

  const browser = await chromium.launch({ headless: true })
  const context = await browser.newContext({
    viewport: { width: 1440, height: 1000 },
    locale: 'en-US',
    reducedMotion: 'reduce',
  })
  await context.addInitScript(() => {
    localStorage.setItem('i18nextLng', 'en')
  })

  const page = await context.newPage()
  const pageErrors = []
  const consoleErrors = []
  const failedResponses = []
  const apiHeaders = {}
  page.on('request', (request) => {
    if (!request.url().startsWith(`${baseUrl}/api/`)) return
    const headers = request.headers()
    for (const key of ['authorization', 'new-api-user']) {
      if (headers[key]) apiHeaders[key] = headers[key]
    }
  })
  page.on('pageerror', (error) => pageErrors.push(error.message))
  page.on('response', (response) => {
    if (response.status() < 400) return
    failedResponses.push({
      path: new URL(response.url()).pathname,
      status: response.status(),
    })
  })
  page.on('console', (message) => {
    if (message.type() === 'error') consoleErrors.push(message.text())
  })

  let originalSetting
  let labels
  try {
    await page.goto(`${baseUrl}/sign-in?redirect=/workspace`)
    await page.getByLabel('Username or Email', { exact: true }).fill(username)
    await page.getByLabel('Password', { exact: true }).fill(password)
    const loginResponsePromise = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === '/api/user/login' &&
        response.request().method() === 'POST'
    )
    await page.getByRole('button', { name: 'Sign in', exact: true }).click()
    const loginResponse = await loginResponsePromise
    assert.equal(
      loginResponse.status(),
      200,
      'The real sign-in request must succeed.'
    )
    const loginResult = await loginResponse.json()
    assert.equal(
      loginResult.success,
      true,
      loginResult.message || 'Sign-in failed.'
    )
    await expect(page).toHaveURL((url) => url.pathname === '/workspace', {
      timeout: 30000,
    })
    await expect(page.locator('header')).toBeVisible({ timeout: 30000 })

    const selfResponse = await page.request.get(`${baseUrl}/api/user/self`, {
      headers: apiHeaders,
    })
    assert.equal(selfResponse.status(), 200)
    const self = await selfResponse.json()
    assert.equal(self.success, true)
    originalSetting = self.data.setting

    await page.route('**/api/user/self', (route) => {
      const request = route.request()
      if (request.method() === 'PUT' && request.postDataJSON()?.language) {
        return route.fulfill({
          json: { success: true, message: '' },
        })
      }
      return route.continue()
    })

    assert.ok(
      failedResponses.every(
        (response) =>
          response.path === '/api/user/auth/refresh' && response.status === 401
      ),
      `Unexpected pre-login response errors: ${JSON.stringify(failedResponses)}`
    )
    assert.ok(
      consoleErrors.every((message) => message.includes('401 (Unauthorized)')),
      `Unexpected pre-login console errors: ${JSON.stringify(consoleErrors)}`
    )
    consoleErrors.length = 0
    failedResponses.length = 0

    const signinToast = page.locator('[data-sonner-toast][data-type="success"]')
    await expect(signinToast).toBeVisible({ timeout: 10000 })
    await signinToast.locator('[data-close-button]').click()
    await expect(page.locator('[data-sonner-toast]')).toHaveCount(0)

    const header = page.locator('header')
    const navigation = header.getByRole('navigation')
    const languageNames = await Promise.all(
      interfaceLanguages.map(async ([, filename]) => {
        const dictionary = JSON.parse(
          await fs.readFile(
            path.join(webRoot, `src/i18n/locales/${filename}.json`),
            'utf8'
          )
        ).translation
        return dictionary['Change language'].replaceAll(
          /[.*+?^${}()|[\]\\]/g,
          '\\$&'
        )
      })
    )
    await header
      .getByRole('button', {
        name: new RegExp(`^(?:${languageNames.join('|')})$`),
      })
      .click()
    await page.getByRole('menuitem', { name: 'English', exact: true }).click()
    await expect(
      navigation.getByRole('link', { name: 'Models', exact: true })
    ).toBeVisible()
    const expectedUrls = [
      `${websiteOrigin}/`,
      `${websiteOrigin}/models`,
      documentationUrl,
      `${websiteOrigin}/rankings`,
    ]
    await expect(navigation.getByRole('link')).toHaveCount(4)
    assert.deepEqual(
      await navigation
        .getByRole('link')
        .evaluateAll((links) => links.map((link) => link.href)),
      expectedUrls
    )
    for (const link of await navigation.getByRole('link').all()) {
      await expect(link).toHaveAttribute('target', '_blank')
      await expect(link).toHaveAttribute('rel', 'noopener noreferrer')
    }

    const modelsDestination = `${websiteOrigin}/models`
    await context.route(modelsDestination, (route) =>
      route.fulfill({
        contentType: 'text/html',
        body: '<h1>Models destination</h1>',
      })
    )
    const [modelsPopup] = await Promise.all([
      context.waitForEvent('page'),
      navigation.getByRole('link', { name: 'Models', exact: true }).click(),
    ])
    await expect(modelsPopup).toHaveURL(modelsDestination)
    await modelsPopup.close()
    await context.unroute(modelsDestination)
    await expect(
      header.getByRole('button', { name: 'Search', exact: true })
    ).toHaveCount(0)
    await expect(
      header.getByRole('button', { name: 'Open theme settings' })
    ).toHaveCount(0)
    await expect(
      header.getByRole('link', { name: 'Go to home' })
    ).toHaveAttribute('href', `${websiteOrigin}/`)

    const headerAppearance = await header.evaluate((element) => {
      const style = getComputedStyle(element)
      return {
        height: style.height,
        position: style.position,
        background: style.backgroundColor,
        border: style.borderBottomWidth,
      }
    })
    assert.equal(headerAppearance.height, '48px')
    assert.equal(headerAppearance.position, 'sticky')
    assert.equal(headerAppearance.border, '1px')
    assert.notEqual(headerAppearance.background, 'rgba(0, 0, 0, 0)')
    await expect(header.locator('[data-token-dazi-wordmark]')).toBeVisible()
    assert.ok(
      await header
        .locator('[data-token-dazi-brand] img')
        .evaluate((image) => image.complete && image.naturalWidth > 0),
      'The final Overlay brand SVG must load.'
    )

    await page.screenshot({
      path: path.join(outputDirectory, 'desktop-light.png'),
    })
    await header.getByRole('button', { name: 'Toggle Sidebar' }).click()
    await expect(header.getByRole('navigation')).toBeVisible()
    await header.getByRole('button', { name: 'Toggle Sidebar' }).click()

    await header.getByRole('button', { name: 'Profile', exact: true }).click()
    await page.getByRole('menuitem', { name: 'Theme Settings' }).click()
    const settings = page.getByRole('dialog', { name: 'Theme Settings' })
    await expect(settings).toBeVisible()
    await expect(
      settings.getByText('Content width', { exact: true })
    ).toBeVisible()
    await settings.getByRole('radio', { name: 'Dark' }).click()
    await expect(page.locator('html')).toHaveClass(/dark/)
    await page.keyboard.press('Escape')
    await expect(settings).toHaveCount(0)
    await expect(
      header.getByRole('button', { name: 'Profile', exact: true })
    ).toBeFocused()
    await page.screenshot({
      path: path.join(outputDirectory, 'desktop-dark.png'),
    })

    for (const width of [320, 375, 419, 420, 421, 768, 1024, 1920]) {
      await page.setViewportSize({ width, height: 900 })
      await expect(header).toBeVisible()
      await expect
        .poll(() =>
          header.evaluate(
            (element) => element.scrollWidth <= element.clientWidth
          )
        )
        .toBe(true)
      assert.equal(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= window.innerWidth
        ),
        true,
        `The Console must not overflow horizontally at ${width}px.`
      )
      if (width < 1024) {
        await expect(navigation).toBeHidden()
        const menuTrigger = header.getByRole('button', { name: 'Open menu' })
        await expect(menuTrigger).toBeVisible()
        await menuTrigger.click()
        await expect(
          page.getByRole('menuitem', { name: 'Models', exact: true })
        ).toHaveAttribute('href', `${websiteOrigin}/models`)
        await page.keyboard.press('Escape')
        await expect(menuTrigger).toBeFocused()
      } else {
        await expect(navigation).toBeVisible()
        await expect(
          header.getByRole('button', { name: 'Open menu' })
        ).toBeHidden()
      }
      if (width < 420) {
        await expect(header.locator('[data-token-dazi-wordmark]')).toBeHidden()
      } else {
        await expect(header.locator('[data-token-dazi-wordmark]')).toBeVisible()
      }
      if (width === 375) {
        await page.screenshot({
          path: path.join(outputDirectory, 'mobile-dark.png'),
        })
      }
    }

    await page.setViewportSize({ width: 1440, height: 1000 })
    labels = JSON.parse(
      await fs.readFile(path.join(webRoot, 'src/i18n/locales/en.json'), 'utf8')
    ).translation
    for (const [code, filename, menuLabel] of interfaceLanguages) {
      await header
        .getByRole('button', { name: labels['Change language'] })
        .click()
      await page.getByRole('menuitem', { name: menuLabel, exact: true }).click()
      labels = JSON.parse(
        await fs.readFile(
          path.join(webRoot, `src/i18n/locales/${filename}.json`),
          'utf8'
        )
      ).translation
      for (const key of ['Home', 'Models', 'Docs', 'Rankings']) {
        await expect(
          navigation.getByRole('link', { name: labels[key], exact: true })
        ).toBeVisible()
      }
      assert.deepEqual(
        await navigation
          .getByRole('link')
          .evaluateAll((links) => links.map((link) => link.href)),
        expectedUrls,
        `Website paths must remain canonical in ${code}.`
      )
      await expect
        .poll(() =>
          header.evaluate(
            (element) => element.scrollWidth <= element.clientWidth
          )
        )
        .toBe(true)
    }
    await header
      .getByRole('button', { name: labels['Change language'] })
      .click()
    await page.getByRole('menuitem', { name: 'English', exact: true }).click()
    await expect(
      navigation.getByRole('link', { name: 'Models', exact: true })
    ).toBeVisible()
    await page.keyboard.press('Control+k')
    await expect(page.getByRole('dialog')).toBeVisible()
    await page.keyboard.press('Escape')

    const finalSelfResponse = await page.request.get(
      `${baseUrl}/api/user/self`,
      {
        headers: apiHeaders,
      }
    )
    assert.deepEqual(
      (await finalSelfResponse.json()).data.setting,
      originalSetting
    )
    assert.deepEqual(pageErrors, [], 'No browser runtime errors are allowed.')
    assert.deepEqual(
      failedResponses,
      [],
      'No signed-in resource failures are allowed.'
    )
    assert.deepEqual(
      consoleErrors,
      [],
      'No browser console errors are allowed.'
    )
    process.stdout.write(
      'PASS: real login, website URLs, Overlay branding/background, desktop/mobile layouts, theme settings/focus, sidebar, seven languages, unchanged account language, command shortcut and clean signed-in console.\n'
    )
  } catch (error) {
    await fs.writeFile(
      path.join(outputDirectory, 'failure.json'),
      JSON.stringify(
        {
          failedResponses,
          pageErrors,
          consoleErrors,
          headerText: await page
            .locator('header')
            .innerText()
            .catch(() => null),
        },
        null,
        2
      )
    )
    await page.screenshot({
      path: path.join(outputDirectory, 'failure.png'),
    })
    throw error
  } finally {
    await browser.close()
  }
}

main().catch((error) => {
  process.stderr.write(`${error.message}\n`)
  process.exitCode = 1
})
