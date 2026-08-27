// @ts-check

const {test, expect} = require('@playwright/test');

test.beforeEach(async ({page}) => {
    await page.goto('/browser-login/');
    await expect(page).toHaveURL(/\/horizon\/$/);
    await expect(page.locator('[data-horizon-view]')).toBeVisible();
    await page.goto('/');
    await expect(page).toHaveURL(/\/horizon\/$/);
});

test('renders the lane and marker contract', async ({page}) => {
    await expect(page.locator('[data-calendar-id="CALBIRTH"] [data-lane-id]')).toHaveCount(3);
    await expect(page.locator('[data-calendar-id="CALHOLID"] [data-lane-id]')).toHaveCount(2);
    await expect(page.locator('[data-calendar-id="CALTRAVL"] [data-lane-id]')).toHaveCount(1);
    await expect(page.locator('[data-calendar-id="CALTRAVL"] [data-marker-id]')).toHaveCount(2);
    await expect(page.locator('[data-calendar-id="CALWORK0"] [data-lane-id]')).toHaveCount(3);
    await expect(page.locator('[data-calendar-id="CALWORK0"] [data-marker-id]')).toHaveCount(3);

    const openLane = page.locator('[data-lane-id="LANWAIT0"]');
    await expect(openLane.locator('[data-marker-id]')).toHaveCount(0);
    const openTrackBox = await openLane.locator('.horizon-lane-track').boundingBox();
    const openLineBox = await openLane.locator('.horizon-lane-line.is-open').boundingBox();
    expect(openTrackBox).not.toBeNull();
    expect(openLineBox).not.toBeNull();
    expect(Math.abs(openLineBox.x - openTrackBox.x)).toBeLessThanOrEqual(1);
    expect(Math.abs((openLineBox.x + openLineBox.width) - (openTrackBox.x + openTrackBox.width))).toBeLessThanOrEqual(1);
    expect(openLineBox.height).toBeGreaterThanOrEqual(12);
    expect(openLineBox.height).toBeLessThanOrEqual(16);

    const openEndStyle = await openLane.locator('.horizon-lane-line').evaluate((line) => getComputedStyle(line, '::after').borderLeftStyle);
    const finiteEndStyle = await page.locator('.horizon-lane-line.is-finite').first().evaluate((line) => getComputedStyle(line, '::after').borderStyle);
    expect(openEndStyle).toBe('solid');
    expect(finiteEndStyle).toContain('solid');
    const continuingEndStyle = await page.locator('[data-lane-id="LANLONG0"] .horizon-lane-line.is-continuing').evaluate((line) => getComputedStyle(line, '::after').borderLeftStyle);
    expect(continuingEndStyle).toBe('solid');

    const markerTargetBox = await page.locator('[data-marker-id]').first().boundingBox();
    const markerDotBox = await page.locator('.horizon-marker-dot').first().boundingBox();
    expect(markerTargetBox).not.toBeNull();
    expect(markerDotBox).not.toBeNull();
    expect(markerTargetBox.width).toBeGreaterThanOrEqual(44);
    expect(markerTargetBox.height).toBeGreaterThanOrEqual(44);
    expect(markerDotBox.width).toBeGreaterThanOrEqual(20);
    expect(markerDotBox.width).toBeLessThanOrEqual(24);
    expect(markerDotBox.height).toBeGreaterThanOrEqual(20);
    expect(markerDotBox.height).toBeLessThanOrEqual(24);

    const boundaryTrackBox = await page.locator('[data-lane-id="LANBOUND"] .horizon-lane-track').boundingBox();
    const boundaryTargetBox = await page.locator('[data-marker-id="EVTBOUND"]').boundingBox();
    expect(boundaryTrackBox).not.toBeNull();
    expect(boundaryTargetBox).not.toBeNull();
    expect(boundaryTargetBox.x).toBeGreaterThanOrEqual(boundaryTrackBox.x - 1);
    expect(boundaryTargetBox.x + boundaryTargetBox.width).toBeLessThanOrEqual(boundaryTrackBox.x + boundaryTrackBox.width + 1);

    await page.locator('[data-marker-id="EVTFLGHT"]').focus();
    await expect(page.locator('[data-marker-details="EVTFLGHT"]')).toBeVisible();
    await expect(page.locator('[data-marker-details="EVTFLGHT"] a', {hasText: 'Event controls'})).toHaveAttribute('href', '/events/?event_id=EVTFLGHT');
    await expect(page.locator('[data-marker-details="EVTFLGHT"] a', {hasText: 'RSVP controls'})).toHaveAttribute('href', '/rsvps/?event_id=EVTFLGHT');
});

test('persists complete calendar visibility for the organizer', async ({context, page}) => {
    const birthdaysToggle = page.locator('[data-calendar-toggle="CALBIRTH"]');
    await birthdaysToggle.uncheck();
    await expect(page.locator('[data-calendar-id="CALBIRTH"]')).toBeHidden();
    await expect(page.locator('[data-calendar-id="CALHOLID"]')).toBeVisible();

    await context.clearCookies();
    await page.goto('/browser-login/');
    await expect(page.locator('[data-calendar-toggle="CALBIRTH"]')).not.toBeChecked();
    await expect(page.locator('[data-calendar-id="CALBIRTH"]')).toBeHidden();
    await expect(page.locator('[data-calendar-control="CALBIRTH"]')).toBeVisible();

    await page.locator('[data-calendar-toggle="CALBIRTH"]').check();
    await expect(page.locator('[data-calendar-id="CALBIRTH"]')).toBeVisible();
});

test('creates, reorders, resolves, and persists calendar lanes', async ({context, page}) => {
    await page.locator('[data-horizon-management] > summary').click();
    const calendarForm = page.locator('form[data-resource-url="/calendars/"]');
    await calendarForm.locator('[name="name"]').fill('Browser Calendar');
    await calendarForm.locator('[name="symbol"]').fill('B');
    await calendarForm.locator('[name="color_token"]').fill('browser-calendar');
    await calendarForm.getByRole('button', {name: 'Create calendar'}).click();

    await expect(page.locator('[data-calendar-toggle]').last()).toHaveAttribute('data-calendar-toggle', /.+/);
    await expect(page.locator('.horizon-calendar-toggle').last()).toContainText('Browser Calendar');
    const calendarID = await page.locator('[data-calendar-toggle]').last().getAttribute('data-calendar-toggle');
    expect(calendarID).toBeTruthy();

    await page.locator('[data-horizon-management] > summary').click();
    const calendarManagement = page.locator(`[data-calendar-management="${calendarID}"]`);
    await calendarManagement.getByRole('button', {name: 'Move calendar up'}).click();
    await expect(page.locator('[data-calendar-toggle]').nth(4)).toHaveAttribute('data-calendar-toggle', calendarID);

    await page.locator('[data-horizon-management] > summary').click();
    const laneForm = page.locator('form[data-resource-url="/lanes/"]');
    await laneForm.locator('[name="calendar_id"]').selectOption(calendarID);
    await laneForm.locator('[name="title"]').fill('Browser open lane');
    await laneForm.getByRole('button', {name: 'Create lane'}).click();
    const openLane = page.locator(`[data-calendar-id="${calendarID}"] [data-lane-id]`, {hasText: 'Browser open lane'});
    await expect(openLane).toBeVisible();
    await expect(openLane).toHaveAttribute('data-lane-open', 'true');

    await page.locator('[data-horizon-management] > summary').click();
    await laneForm.locator('[name="calendar_id"]').selectOption(calendarID);
    await laneForm.locator('[name="title"]').fill('Browser finite lane');
    await laneForm.locator('[name="kind"]').selectOption('finite');
    const localEnd = await page.evaluate(() => {
        const value = new Date(Date.now() + 7 * 24 * 60 * 60 * 1000);
        value.setMinutes(value.getMinutes() - value.getTimezoneOffset());
        return value.toISOString().slice(0, 16);
    });
    await laneForm.locator('[name="ends_at"]').fill(localEnd);
    await laneForm.getByRole('button', {name: 'Create lane'}).click();
    const finiteLane = page.locator(`[data-calendar-id="${calendarID}"] [data-lane-id]`, {hasText: 'Browser finite lane'});
    await expect(finiteLane).toBeVisible();
    await expect(finiteLane).toHaveAttribute('data-lane-open', 'false');

    await finiteLane.locator('.horizon-lane-controls > summary').click();
    await finiteLane.getByRole('button', {name: 'Move lane up'}).click();
    await expect(page.locator(`[data-calendar-id="${calendarID}"] .horizon-lane-label > span:nth-child(2)`)).toHaveText(['Browser finite lane', 'Browser open lane']);

    await context.clearCookies();
    await page.goto('/browser-login/');
    await expect(page.locator('[data-calendar-toggle]').nth(4)).toHaveAttribute('data-calendar-toggle', calendarID);
    await expect(page.locator(`[data-calendar-id="${calendarID}"] .horizon-lane-label > span:nth-child(2)`).first()).toHaveText('Browser finite lane');

    const persistedOpenLane = page.locator(`[data-calendar-id="${calendarID}"] [data-lane-id]`, {hasText: 'Browser open lane'});
    await persistedOpenLane.locator('.horizon-lane-controls > summary').click();
    await persistedOpenLane.getByRole('button', {name: 'Resolve lane'}).click();
    await expect(page.locator(`[data-calendar-id="${calendarID}"] [data-lane-id]`, {hasText: 'Browser open lane'})).toHaveAttribute('data-lane-open', 'false');
});

test('supports keyboard pan, scale, visibility, and marker selection', async ({page}) => {
    const viewport = page.locator('[data-horizon-viewport]');
    const board = page.locator('[data-horizon-board]');
    await viewport.focus();
    const initialBoardWidth = await board.evaluate((element) => element.scrollWidth);
    await page.keyboard.press('=');
    const scaledBoardWidth = await board.evaluate((element) => element.scrollWidth);
    expect(scaledBoardWidth).toBeGreaterThan(initialBoardWidth);

    await page.keyboard.press('l');
    await expect.poll(() => viewport.evaluate((element) => element.scrollLeft)).toBeGreaterThan(0);
    await page.keyboard.press('2');
    await expect(page.locator('[data-calendar-id="CALHOLID"]')).toBeHidden();
    await page.keyboard.press('j');
    await expect(page.locator('[data-marker-id].is-selected')).toHaveCount(1);
    await expect(page.locator('[data-marker-id].is-selected')).toBeFocused();
    await page.keyboard.press('2');
    await expect(page.locator('[data-calendar-id="CALHOLID"]')).toBeVisible();
});

test('restores independent-event calendar selection after cancel', async ({page}) => {
    await page.goto('/events/');
    await page.locator('#globalNewEventButton').click();
    const anchorSelect = page.locator('#anchorEventSelect');
    const calendarSelect = page.locator('#calendarSelect');
    await anchorSelect.selectOption({index: 1});
    await expect(calendarSelect).toBeDisabled();

    await page.locator('#cancelNewEventButton').click();
    await page.locator('#globalNewEventButton').click();
    await expect(anchorSelect).toHaveValue('');
    await expect(calendarSelect).toBeEnabled();
});

test('renders the interactive view at the supported mobile width', async ({page}) => {
    await page.setViewportSize({width: 390, height: 844});
    await expect(page.getByRole('heading', {name: 'Horizon'})).toBeVisible();
    await expect(page.locator('[data-horizon-viewport]')).toBeVisible();
    await expect(page.locator('[data-lane-id="LANWAIT0"] .horizon-lane-line.is-open')).toBeVisible();
    const markerTargetBox = await page.locator('[data-marker-id]').first().boundingBox();
    expect(markerTargetBox).not.toBeNull();
    expect(markerTargetBox.width).toBeGreaterThanOrEqual(44);
    expect(markerTargetBox.height).toBeGreaterThanOrEqual(44);
    const documentWidth = await page.evaluate(() => document.documentElement.scrollWidth);
    expect(documentWidth).toBeLessThanOrEqual(390);
    const navigationBox = await page.locator('.fixed-navbar').boundingBox();
    const headingBox = await page.getByRole('heading', {name: 'Horizon'}).boundingBox();
    expect(navigationBox).not.toBeNull();
    expect(headingBox).not.toBeNull();
    expect(headingBox.y).toBeGreaterThanOrEqual(navigationBox.y + navigationBox.height);
});
