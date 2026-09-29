// @ts-check
const {test, expect} = require('@playwright/test');

test('loads and clears the protected workspace only on shared authentication events', async ({page}) => {
    await page.goto('/browser-login/');
    const protectedRequests = [];
    page.on('request', (request) => {
        if (/\/(horizon|calendars|lanes|events|venues|organizers)\//.test(new URL(request.url()).pathname)) {
            protectedRequests.push(request.url());
        }
    });
    await page.goto('/');
    const frame = page.locator('[data-workspace-frame]');
    await expect(page.locator('mpr-header')).toBeVisible();
    await expect(frame).toBeHidden();
    await expect(frame).not.toHaveAttribute('src');
    expect(protectedRequests).toEqual([]);
    // The external authentication owner supplies this documented event.
    await page.evaluate(() => document.dispatchEvent(new CustomEvent('mpr-ui:auth:authenticated')));
    await expect(frame).toBeVisible();
    await expect(page.frameLocator('[data-workspace-frame]').locator('[data-horizon-view]')).toBeVisible();
    await page.evaluate(() => document.dispatchEvent(new CustomEvent('mpr-ui:auth:unauthenticated')));
    await expect(frame).toBeHidden();
    await expect(frame).not.toHaveAttribute('src');
    await expect(page.locator('[data-workspace-message]')).toBeVisible();
    await page.goto('/#events');
    await expect(frame).not.toHaveAttribute('src');
});

test('runs Calendar consent in the top window from the shared shell', async ({page}) => {
    await page.goto('/browser-shell-login/');
    if (await page.locator('[data-horizon-setup]').count()) {
        await page.getByRole('button', {name:'Start Horizon',exact:true}).click();
    }
    await page.goto('/');
    await expect(page.locator('mpr-header')).toBeVisible();
    await page.evaluate(() => document.dispatchEvent(new CustomEvent('mpr-ui:auth:authenticated')));
    const workspace=page.frameLocator('[data-workspace-frame]');
    await expect(workspace.locator('[data-horizon-view]')).toBeVisible();
    await workspace.getByRole('button',{name:'Settings',exact:true}).click();
    await workspace.getByRole('tab',{name:'Integrations',exact:true}).click();
    await workspace.getByRole('button',{name:'Connect Google Calendar',exact:true}).click();
    await expect(page.locator('[data-calendar-confirmation]')).toBeVisible();
    await expect(page).toHaveURL(/\/calendar-connection-callbacks\/google\//);
    const createdConnection = page.waitForResponse(response => response.request().method() === 'POST' && new URL(response.url()).pathname === '/calendar-connections/');
    await page.getByRole('button',{name:'Create connection',exact:true}).click();
    const connectionResponse = await createdConnection;
    expect(connectionResponse.status(), await connectionResponse.text()).toBe(202);
    await expect(page).toHaveURL(/\/horizon\/#settings\/integrations$/);
    await expect(page.locator('[data-settings-dialog]')).toBeVisible();
    await expect(page.locator('[data-calendar-task-state]')).toHaveText('Complete', {timeout: 10000});
});
